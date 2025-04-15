#!/usr/bin/env python3

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

import time
import http.client
import urllib.parse
import json
import os
import yaml
import argparse
import subprocess
from datetime import datetime, timedelta

abspath = os.path.abspath(__file__)
curr_dir = os.path.dirname(abspath)
QUERY_PERIOD = 7 # days
STEP = "60" # seconds

# Function to get metrics from the Metrics Server
def get_metrics(namespace, istio=False):
    result = {}
    query_parameters = []
    if istio:
        query_parameters.append('k8s_container_name="istio-proxy"')
    else:
        query_parameters.append('k8s_container_name!="istio-proxy"')
    if namespace:
        query_parameters.append(f'k8s_namespace_name="{namespace}"')
    else:
        query_parameters.append(f'k8s_namespace_name!="kube-system"')

    query_parameter_str = ", ".join(query_parameters)
    end_time = datetime.now()
    start_time = end_time - timedelta(days=QUERY_PERIOD)

    # CPU
    query=f"max(container_cpu_utilization{{ {query_parameter_str} }}*1000) by (k8s_pod_name, k8s_container_name)"
    resp = prometheus_query_range(query, start_time, end_time, STEP)
    if "status" in resp and resp["status"] == "success":
        metrics_result = resp.get("data", {}).get("result", [])
        for metric in metrics_result:
            values = metric.get("values", [])
            metric_data = metric.get("metric", {})
            if not metric_data:
                continue
            pod_name = metric_data.get("k8s_pod_name", "")
            container_name = metric_data.get("k8s_container_name", "")
            if not pod_name or not container_name:
                continue
            max_cpu, min_cpu = 100, 999999
            for value in values:
                cpu = round(float(value[1]))
                max_cpu = max(max_cpu, cpu)
                min_cpu = min(min_cpu, cpu)
            max_cpu = max_cpu * 2
            if pod_name in result:
                result[pod_name][container_name] = {
                    "cpu": {
                        "max": max_cpu,
                        "min": min_cpu
                    }
                }
            else:
                result[pod_name] = {
                    container_name: {
                        "cpu": {
                            "max": max_cpu,
                            "min": min_cpu
                        }
                    }
                }

    # Memory
    query=f"max(container_memory_working_set{{ {query_parameter_str} }}/1000000) by (k8s_pod_name, k8s_container_name)"
    resp = prometheus_query_range(query, start_time, end_time, STEP)
    if "status" in resp and resp["status"] == "success":
        metrics_result = resp.get("data", {}).get("result", [])
        for metric in metrics_result:
            values = metric.get("values", [])
            metric_data = metric.get("metric", {})
            if not metric_data:
                continue
            pod_name = metric_data.get("k8s_pod_name", "")
            container_name = metric_data.get("k8s_container_name", "")
            max_memory, min_memory = 128, 999999
            for value in values:
                memory = round(float(value[1]))
                max_memory = max(max_memory, memory)
                min_memory = min(min_memory, memory)
            max_memory = max_memory * 2
            if pod_name in result:
                result[pod_name][container_name]["memory"] = {
                    "max": max_memory,
                    "min": min_memory
                }
            else:
                result[pod_name] = {
                    container_name: {
                        "memory": {
                            "max": max_memory,
                            "min": min_memory
                        }
                    }
                }
    flat = []
    for pod_name, containers in result.items():
        for container_name, metrics in containers.items():
            flat.append({
                "pod_name": pod_name,
                "container_name": container_name,
                "cpu_max": metrics["cpu"]["max"],
                "cpu_min": metrics["cpu"]["min"],
                "memory_max": metrics["memory"]["max"],
                "memory_min": metrics["memory"]["min"]
            })
    return flat

def load_mappings():
    with open(f"{curr_dir}/mapping.yaml", 'r') as file:
        mapping = yaml.load(file, Loader=yaml.FullLoader)
    return mapping

def get_resource_config(namespace):
    mapping = load_mappings()
    metrics = get_metrics(namespace)
    resoruce_configs = {} # key -> requests and limits
    if pod_prefixes not in mapping:
        return {}, { "cpu_request": 0, "memory_request": 0, "cpu_limit": 0, "memory_limit": 0 }
    pod_prefixes = mapping.get(namespace, {})
    for item in metrics:
        pod_name = item["pod_name"]
        container_config_mapping = {}
        for pod_prefix in pod_prefixes:
            if pod_name.startswith(pod_prefix):
                container_config_mapping = mapping[namespace][pod_prefix]
                break
        else:
            # nothing found
            continue

        container_name = item['container_name']
        cpu_max = item['cpu_max']
        memory_max = item['memory_max']
        cpu_min = item['cpu_min']
        memory_min = item['memory_min']

        if container_name in container_config_mapping:
            config_key = container_config_mapping[container_name]
            if config_key not in resoruce_configs:
                resoruce_configs[config_key] = {
                    "cpu_request": cpu_min,
                    "memory_request": memory_min,
                    "cpu_limit": cpu_max,
                    "memory_limit": memory_max
                }
            else:
                resoruce_configs[config_key]["cpu_request"] = max(resoruce_configs[config_key]["cpu_request"], cpu_min)
                resoruce_configs[config_key]["memory_request"] = max(resoruce_configs[config_key]["memory_request"], memory_min)
                resoruce_configs[config_key]["cpu_limit"] = max(resoruce_configs[config_key]["cpu_limit"], cpu_max)
                resoruce_configs[config_key]["memory_limit"] = max(resoruce_configs[config_key]["memory_limit"], memory_max)

    istio_usage = {
        "cpu_request": 0,
        "memory_request": 0,
        "cpu_limit": 0,
        "memory_limit": 0,
    }
    metrics = get_metrics(None, istio=True)
    for item in metrics:
        cpu_max = item['cpu_max']
        memory_max = item['memory_max']
        cpu_min = item['cpu_min']
        memory_min = item['memory_min']

        istio_usage["cpu_request"] = max(istio_usage["cpu_request"], cpu_min)
        istio_usage["memory_request"] = max(istio_usage["memory_request"], memory_min)
        istio_usage["cpu_limit"] = max(istio_usage["cpu_limit"], cpu_max)
        istio_usage["memory_limit"] = max(istio_usage["memory_limit"], memory_max)

    return resoruce_configs, istio_usage

def convert_to_nested_dict(flat_dict):
    nested_dict = {}
    for key, value in flat_dict.items():
        keys = key.split('.')
        d = nested_dict
        for k in keys[:-1]:
            if k not in d:
                d[k] = {}
            d = d[k]
        cpu_request = value["cpu_request"]
        memory_request = value["memory_request"]
        cpu_limit = value["cpu_limit"]
        memory_limit = value["memory_limit"]
        d[keys[-1]] = {
            "requests": {
                "cpu": f"{cpu_request}m",
                "memory": f"{memory_request}Mi"
            },
            "limits": {
                "cpu": f"{cpu_limit}m",
                "memory": f"{memory_limit}Mi"
            }
        }
    return nested_dict

def start_mimir_gw_port_forwarding():
    command = ["kubectl", "port-forward", "-n", "orch-platform", "svc/orchestrator-observability-mimir-gateway", "8181"]
    process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    time.sleep(3)
    return process

def prometheus_query_range(query: str, start: datetime, end: datetime, step: str):
    host="localhost:8181"
    start = round(start.timestamp())
    end = round(end.timestamp())
    params = {"query": query, "start": start, "end": end, "step": step}
    headers = {
        "Content-type": "application/x-www-form-urlencoded",
        "user-agent": "Grafana/11.4.0",
        "x-datasource-uid": "orchestrator-mimir",
        "x-grafana-org-id": "1",
        "x-scope-orgid": "orchestrator-system"
    }
    connection = http.client.HTTPConnection(host)
    connection.request("POST", "/prometheus/api/v1/query_range", urllib.parse.urlencode(params), headers)
    response = connection.getresponse()
    data = response.read()
    connection.close()
    return json.loads(data)

def main():
    parser = argparse.ArgumentParser(description="Manage resource limits")
    subparsers = parser.add_subparsers(dest="command", required=True)
    subparsers.add_parser("freeze", help="Freeze resource limits")
    subparsers.add_parser("unfreeze", help="Unfreeze resource limits")
    parser.add_argument("--dry", action="store_true", help="Dry run")
    args = parser.parse_args()

    command = ["helm", "list", "-A", "-o", "yaml"]
    helm_list = subprocess.run(command, stdout=subprocess.PIPE)
    helm_list = helm_list.stdout.decode("utf-8")
    helm_list = yaml.load(helm_list, Loader=yaml.FullLoader)
    helm_release_namespace = None
    for release in helm_list:
        if release["name"] == "root-app":
            helm_release_namespace = release["namespace"]
            break
    else:
        print("root-app release not found")
        return

    # Get values from root-app
    command = ["helm", "get", "values", "root-app", "-n", helm_release_namespace]
    with open("/tmp/root-app-values.yaml", "w") as file:
        subprocess.run(command, stdout=file)

    resource_configs_to_override = {}
    if args.command == "freeze":
        print("Freezing resource limits...")
        namespaces = [
            "orch-app",
            "orch-platform",
            "orch-infra",
            "cert-manager",
            "istio-system",
            "kyverno",
            "orch-boots",
            "orch-cluster",
            "orch-database",
            "orch-gateway",
            "orch-harbor",
            "orch-iam",
            "orch-secret",
            "orch-sre",
            "orch-ui"
        ]
        maximum_istio_usage = {
            "cpu_request": 0,
            "memory_request": 0,
            "cpu_limit": 0,
            "memory_limit": 0
        }
        pf = start_mimir_gw_port_forwarding()
        try:
            for ns in namespaces:
                resource_config, istio_usage = get_resource_config(ns)
                if resource_config is None:
                    print(f"Failed to get metrics for namespace: {ns}")
                    continue
                maximum_istio_usage["cpu_request"] = max(maximum_istio_usage["cpu_request"], istio_usage["cpu_request"])
                maximum_istio_usage["memory_request"] = max(maximum_istio_usage["memory_request"], istio_usage["memory_request"])
                maximum_istio_usage["cpu_limit"] = max(maximum_istio_usage["cpu_limit"], istio_usage["cpu_limit"])
                maximum_istio_usage["memory_limit"] = max(maximum_istio_usage["memory_limit"], istio_usage["memory_limit"])

                for rc, res in resource_config.items():
                    if rc not in resource_configs_to_override:
                        resource_configs_to_override[rc] = res
                    else:
                        resource_configs_to_override[rc]["cpu"] = max(resource_configs_to_override[rc]["cpu"], res["cpu"])
                        resource_configs_to_override[rc]["memory"] = max(resource_configs_to_override[rc]["memory"], res["memory"])
            resource_configs_to_override["argo.resources.istiod.global.proxy"] = maximum_istio_usage
            resource_configs_to_override = convert_to_nested_dict(resource_configs_to_override)
        finally:
            pf.kill()
    elif args.command == "unfreeze":
        print("Unfreezing resource limits...")
        mapping = load_mappings()
        unfreeze_configs = {}
        for _, prefixes in mapping.items():
            for _, container_config_mapping in prefixes.items():
                for _, config_key in container_config_mapping.items():
                    unfreeze_configs[config_key] = {
                        "cpu_request": 1,
                        "memory_request": 1,
                        "cpu_limit": 64000,
                        "memory_limit": 65536
                    }
        resource_configs_to_override = convert_to_nested_dict(unfreeze_configs)
    else:
        print(f"Invalid command: {args.command}")
        return

    with open('/tmp/resource-config.yaml', 'w') as file:
        yaml.dump(resource_configs_to_override, file)

    command = [
        "helm",
        "upgrade",
        "root-app",
        "argocd/root-app",
        "-n", helm_release_namespace,
        "-f", "/tmp/root-app-values.yaml",
        "-f", "/tmp/resource-config.yaml"
    ]
    if not args.dry:
        subprocess.run(command)
    else:
        print("Dry run:")
        print(" ".join(command))

if __name__ == "__main__":
    main()
