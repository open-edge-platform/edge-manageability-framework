# SPDX-FileCopyrightText: (C) 2026 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

"""
======================================================================================
OrchestratorK8sDiagnosticsUtility: Full Cluster Diagnostics For CI/CD & Production Ops
======================================================================================

Script Purpose:
---------------
Perform comprehensive, automated Kubernetes cluster diagnostics for any environment:
cloud, on-premises, edge, or kind clusters. Designed for DevOps teams, SREs, support
engineers, and CI/CD pipelines to quickly identify and report cluster issues.

What This Tool Checks:
----------------------
Always Captured (Critical Issues):
✓ Pod health (error states, crash loops, image pull failures)
✓ Pod restart counts (above threshold)
✓ Deployment & StatefulSet readiness (stuck rollouts, insufficient replicas)
✓ ArgoCD application health and sync status (GitOps)
✓ Node health and availability
✓ Job completion status

With --advanced Flag (Verbose Diagnostics):
✓ Full Kubernetes events listing per namespace
✓ Policy violations from events
✓ Probe failures (readiness, liveness, startup)
✓ PersistentVolumeClaim binding status

Outputs Generated:
------------------
1. Markdown report (diagnostics_full_<timestamp>.md) - Always generated
   • Full kubectl describe output for all failing resources
   • Complete event details and diagnostics
   
2. HTML report (diagnostics_full_<timestamp>.html) - Optional with --output-html
   • Same content as Markdown in browser-friendly format
   • Collapsible sections for kubectl describe output
   
3. JSON summary (diagnostics_full_<timestamp>.json) - Optional with --output-json
   • Structured, machine-readable format for automation
   • Includes error details: reason, message, last_event, container_state, timestamps
   • Deployment/StatefulSet conditions with status, reason, message
   • Automated recommendations for common issues
   • Note: kubectl describe output excluded to keep JSON lightweight

Usage Examples:
---------------
=== CI/CD Automated Diagnostics ===
# Recommended for automated pipelines - clean, actionable issues only:
python ci/orch_k8s_diagnostics.py --errors-only --output-html --output-json

=== Daily Operations & Health Checks ===
# Quick health check - identify issues at a glance (generates HTML report):
python ci/orch_k8s_diagnostics.py --errors-only --output-html

# Scheduled daily health check with JSON output for monitoring tools:
python ci/orch_k8s_diagnostics.py --errors-only --output-json
# Tip: Parse summary.json for automated alerting (check "has_errors" field)

# Comprehensive cluster status review (includes all namespaces, even healthy ones):
python ci/orch_k8s_diagnostics.py --output-html

=== Troubleshooting & Root Cause Analysis ===
# Deep dive with all events, probe failures, and policy violations:
python ci/orch_k8s_diagnostics.py --advanced --output-html

# Maximum detail - includes pod logs for failed containers:
python ci/orch_k8s_diagnostics.py --advanced --include-logs --output-html

# Custom restart threshold (flag pods with 3+ restarts instead of default 5):
python ci/orch_k8s_diagnostics.py --restart-threshold 3 --errors-only --output-html

=== Operational Workflows ===
# Before maintenance window - capture baseline cluster state:
python ci/orch_k8s_diagnostics.py --output-html --output-json
# Save the diagnostics_full_*.html for comparison after maintenance

# After deployment - verify application health:
python ci/orch_k8s_diagnostics.py --errors-only --output-html
# Review HTML report to ensure no new issues were introduced

# Customer support escalation - gather comprehensive diagnostics:
python ci/orch_k8s_diagnostics.py --advanced --include-logs --output-html --output-json
# Share diagnostics_full_*.html with support team, summary.json for ticket automation

Command-Line Options:
---------------------
--restart-threshold N   Minimum restart count to flag a pod (default: 5)
--errors-only           Show only problematic namespaces, hide healthy ones
--advanced              Include verbose diagnostics (events, policy violations, probe failures, PVCs)
--include-logs          Include pod logs (last 20 lines) for failed/restarting pods
--output-html           Generate HTML report for sharing with teams
--output-json           Generate JSON summary for automation/alerting

Key Benefits:
-------------
• Zero configuration - works with any Kubernetes cluster immediately
• Dynamic namespace discovery - no hard-coded namespace lists to maintain
• Comprehensive checks - all diagnostics run automatically, no mode flags needed
• Multiple output formats - Markdown for viewing, HTML for sharing, JSON for automation
• Fast execution - completes in ~10 seconds for typical clusters
• CI/CD ready - designed for integration into GitHub Actions, Jenkins, etc.

======================================================================================
"""

import subprocess
import sys
import argparse
import json
from datetime import datetime

# --------------------------------------------------------------------------------------
# SHELL UTILITIES & CLUSTER QUERIES
# --------------------------------------------------------------------------------------


def run(cmd):
    """Run a shell command and return stdout, handling errors gracefully."""
    try:
        return subprocess.check_output(cmd, shell=True, timeout=120).decode(
            "utf-8", errors="replace"
        )
    except subprocess.CalledProcessError as exc:
        return f"Error: {cmd}\nreturncode: {exc.returncode}\n{exc.output.decode('utf-8', errors='replace')}"
    except (subprocess.TimeoutExpired, OSError) as exc:
        return f"Error: {cmd}\n{exc}"


def get_all_namespaces():
    """
    Dynamically discover all namespaces in the cluster.
    
    Returns:
        list: Sorted list of namespace names (strings)
        
    Note: This eliminates the need for hard-coded namespace lists, making the
          utility work with any cluster configuration without maintenance.
    """
    raw = run("kubectl get ns -o jsonpath='{.items[*].metadata.name}'")
    return sorted([ns for ns in raw.replace("'", "").strip().split() if ns])


def check_argocd_apps():
    """
    Check ArgoCD application health and sync status across all namespaces.
    
    Flags applications as unhealthy if:
    - Health status is not "Healthy" or "Progressing"
    - Sync status is not "Synced"
    
    Captures deep diagnostic details:
    - Validation errors (invalid versions, missing fields)
    - Resource-level failures (which specific resources failed)
    - Sync operation errors
    - Manifest parsing errors
    - kubectl describe output for unhealthy applications
    
    Returns:
        list: List of dicts containing unhealthy app details with comprehensive diagnostics
    """
    issues = []
    apps = run("kubectl get applications -A -o json 2>/dev/null || echo '{}'")
    try:
        apps_json = json.loads(apps)
        for app in apps_json.get("items", []):
            name = app["metadata"]["name"]
            namespace = app["metadata"]["namespace"]
            status = app.get("status", {})
            health_obj = status.get("health", {})
            sync_obj = status.get("sync", {})
            
            health = health_obj.get("status", "Unknown")
            sync = sync_obj.get("status", "Unknown")

            if health not in ["Healthy", "Progressing"] or sync != "Synced":
                # Collect detailed diagnostic messages
                messages = []
                
                # 1. Conditions (validation errors, comparison errors, etc.)
                conditions = status.get("conditions", [])
                for cond in conditions:
                    cond_type = cond.get("type", "")
                    cond_msg = cond.get("message", "")
                    if cond_type and cond_msg:
                        # These often contain version mismatch, invalid field errors
                        messages.append(f"[{cond_type}] {cond_msg}")
                
                # 2. Operation State - Sync operation details
                op_state = status.get("operationState", {})
                if op_state:
                    # Overall operation message
                    if op_state.get("message"):
                        messages.append(f"Sync: {op_state['message']}")
                    
                    # Phase and status
                    phase = op_state.get("phase", "")
                    if phase and phase not in ["Succeeded", "Running"]:
                        messages.append(f"Phase: {phase}")
                    
                    # Sync result - resource-level failures
                    sync_result = op_state.get("syncResult", {})
                    if sync_result:
                        # Resources that failed
                        resources = sync_result.get("resources", [])
                        failed_resources = [
                            r for r in resources 
                            if r.get("status") not in ["Synced", "Healthy"]
                        ]
                        if failed_resources:
                            for res in failed_resources[:3]:  # Show first 3 failures
                                res_name = res.get("name", "unknown")
                                res_kind = res.get("kind", "")
                                res_msg = res.get("message", "")
                                if res_msg:
                                    messages.append(f"Resource {res_kind}/{res_name}: {res_msg}")
                
                # 3. Resource statuses - individual resource health
                resources = status.get("resources", [])
                unhealthy_resources = [
                    r for r in resources 
                    if r.get("health", {}).get("status") not in ["Healthy", "Progressing", None]
                ]
                for res in unhealthy_resources[:3]:  # Show first 3 unhealthy
                    res_name = res.get("name", "unknown")
                    res_kind = res.get("kind", "")
                    res_health = res.get("health", {})
                    res_msg = res_health.get("message", "")
                    if res_msg and res_msg not in [m for m in messages]:  # Avoid duplicates
                        messages.append(f"{res_kind}/{res_name}: {res_msg}")
                
                # 4. Health message
                if health_obj.get("message"):
                    health_msg = health_obj["message"]
                    if health_msg not in [m for m in messages]:
                        messages.append(f"Health: {health_msg}")
                
                # 5. Sync revision info
                if sync != "Synced":
                    revision = sync_obj.get("revision", "")
                    if revision:
                        messages.append(f"Target Revision: {revision[:12]}")
                
                # 6. Source information (for debugging version issues)
                source = status.get("source", {})
                if source:
                    repo_url = source.get("repoURL", "")
                    target_rev = source.get("targetRevision", "")
                    path = source.get("path", "")
                    if repo_url and target_rev:
                        messages.append(f"Source: {repo_url.split('/')[-1]}@{target_rev} path={path}")
                
                # Fallback to summary message
                if not messages:
                    summary_msg = status.get("summary", {}).get("message", "")
                    if summary_msg:
                        messages.append(summary_msg)
                    else:
                        messages.append("No detailed error message available")
                
                # Capture kubectl describe for unhealthy application (last 100 lines for brevity)
                describe_output = run(f"kubectl describe application {name} -n {namespace}")
                describe_truncated = "\n".join(describe_output.splitlines()[-100:])
                
                issues.append({
                    "name": name,
                    "namespace": namespace,
                    "health": health,
                    "sync": sync,
                    "message": " | ".join(messages),
                    "describe": describe_truncated
                })
    except (json.JSONDecodeError, KeyError):
        pass
    return issues


def check_deployment_readiness(ns):
    """
    Validate Deployment and StatefulSet readiness in a namespace.
    
    Detects:
    - Deployments/StatefulSets with fewer ready replicas than desired
    - Stuck rollouts or scaling issues
    - Provides rollout reason from Progressing condition when available
    - Captures kubectl describe output for problematic deployments
    
    Args:
        ns (str): Namespace to check
        
    Returns:
        list: List of dicts with readiness issues (type, name, expected, ready, reason, describe)
    """
    issues = []

    # Check Deployments
    deps = run(f"kubectl get deployment -n {ns} -o json 2>/dev/null || echo '{{}}'")
    try:
        for dep in json.loads(deps).get("items", []):
            spec_replicas = dep["spec"].get("replicas", 0)
            if spec_replicas == 0:
                continue
            status = dep.get("status", {})
            ready = status.get("readyReplicas", 0)
            if ready < spec_replicas:
                conditions = status.get("conditions", [])
                prog_cond = next((c for c in conditions if c["type"] == "Progressing"), None)
                dep_name = dep["metadata"]["name"]
                describe_output = run(f"kubectl describe deployment {dep_name} -n {ns}")
                describe_truncated = "\n".join(describe_output.splitlines()[-100:])
                
                # Extract structured conditions for JSON
                structured_conditions = []
                for cond in conditions:
                    structured_conditions.append({
                        "type": cond.get("type", ""),
                        "status": cond.get("status", ""),
                        "reason": cond.get("reason", ""),
                        "message": cond.get("message", ""),
                        "lastUpdateTime": cond.get("lastUpdateTime", "")
                    })
                
                issues.append({
                    "type": "Deployment",
                    "name": dep_name,
                    "namespace": ns,
                    "expected": spec_replicas,
                    "ready": ready,
                    "available": status.get("availableReplicas", 0),
                    "reason": prog_cond.get("reason", "Unknown") if prog_cond else "Unknown",
                    "describe": describe_truncated,
                    # Structured fields for JSON/automation
                    "conditions": structured_conditions,
                    "last_update_time": status.get("observedGeneration", "")
                })
    except (json.JSONDecodeError, KeyError):
        pass

    # Check StatefulSets
    sts = run(f"kubectl get statefulset -n {ns} -o json 2>/dev/null || echo '{{}}'")
    try:
        for st in json.loads(sts).get("items", []):
            spec_replicas = st["spec"].get("replicas", 0)
            if spec_replicas == 0:
                continue
            ready = st.get("status", {}).get("readyReplicas", 0)
            if ready < spec_replicas:
                sts_name = st["metadata"]["name"]
                describe_output = run(f"kubectl describe statefulset {sts_name} -n {ns}")
                describe_truncated = "\n".join(describe_output.splitlines()[-100:])
                
                # Extract structured conditions for JSON
                st_status = st.get("status", {})
                conditions = st_status.get("conditions", [])
                structured_conditions = []
                for cond in conditions:
                    structured_conditions.append({
                        "type": cond.get("type", ""),
                        "status": cond.get("status", ""),
                        "reason": cond.get("reason", ""),
                        "message": cond.get("message", ""),
                        "lastUpdateTime": cond.get("lastTransitionTime", "")
                    })
                
                issues.append({
                    "type": "StatefulSet",
                    "name": sts_name,
                    "namespace": ns,
                    "expected": spec_replicas,
                    "ready": ready,
                    "available": ready,
                    "describe": describe_truncated,
                    # Structured fields for JSON/automation
                    "conditions": structured_conditions,
                    "current_revision": st_status.get("currentRevision", ""),
                    "update_revision": st_status.get("updateRevision", "")
                })
    except (json.JSONDecodeError, KeyError):
        pass

    return issues


def check_pvc_status(ns):
    """
    Check PersistentVolumeClaim (PVC) status in a namespace.
    
    Identifies PVCs that are not in "Bound" state, which may indicate:
    - No matching PersistentVolume available
    - StorageClass issues
    - Volume provisioning failures
    
    Args:
        ns (str): Namespace to check
        
    Returns:
        list: List of dicts with PVC issues (pvc name, status, namespace)
    """
    issues = []
    pvcs = run(f"kubectl get pvc -n {ns} -o wide 2>/dev/null || echo ''")
    lines = pvcs.splitlines()
    if len(lines) <= 1:
        return issues
    for line in lines[1:]:
        cols = line.split()
        if len(cols) > 1 and cols[1] != "Bound":
            issues.append({"type": "PVC", "pvc": cols[0], "status": cols[1], "namespace": ns})
    return issues


def parse_args():
    """Parse command-line arguments: see help for details."""
    parser = argparse.ArgumentParser(description="Enhanced K8s Diagnostics Utility")
    parser.add_argument(
        "--restart-threshold",
        type=int,
        default=5,
        help="Number of restarts before considering pod problematic (default: 5)",
    )
    parser.add_argument(
        "--errors-only",
        action="store_true",
        help="Only show diagnostics for namespaces with problems.",
    )
    parser.add_argument(
        "--advanced",
        action="store_true",
        help="Include verbose diagnostics: events, policy violations, probe failures, PVCs.",
    )
    parser.add_argument(
        "--output-html",
        action="store_true",
        help="Write diagnostics report to HTML file.",
    )
    parser.add_argument(
        "--output-json",
        action="store_true",
        help="Write machine-readable summary.json with diagnostics data.",
    )
    parser.add_argument(
        "--include-logs",
        action="store_true",
        help="Include logs for failed/frequent-restart pods.",
    )
    return parser.parse_args()


# --------------------------------------------------------------------------------------
# NAMESPACE DIAGNOSTICS & LOGICS
# --------------------------------------------------------------------------------------


# pylint: disable=too-many-locals,too-many-branches,too-many-statements
def gather_namespace_diagnostics(ns, restart_threshold, errors_only, include_logs, advanced):
    """
    Perform comprehensive diagnostics for a single namespace.
    
    Always Checked:
    - Pod error states (CrashLoopBackOff, ImagePullBackOff, Error)
    - Pod restart counts (compared against threshold)
    - Deployment/StatefulSet readiness
    
    With --advanced flag:
    - Full Kubernetes events listing
    - Policy violations in events
    - Probe failures (readiness, liveness, startup)
    - PVC binding status
    
    Args:
        ns (str): Namespace to diagnose
        restart_threshold (int): Minimum restarts to flag a pod
        errors_only (bool): If True, return empty markdown for healthy namespaces
        include_logs (bool): If True, capture logs for problematic pods
        advanced (bool): If True, include events, violations, probes, PVCs
        
    Returns:
        dict: Contains issues, restarts, violations, markdown, html, logs, etc.
    """
    # --- Fetch cluster data ---
    pods = run(f"kubectl get pods -n {ns} -o wide")
    deployments = run(f"kubectl get deployment -n {ns}")
    pods_lines = pods.splitlines()[1:] if len(pods.splitlines()) > 1 else []

    issues, restarts, policy_violations, probe_failures, pod_logs = [], [], [], [], {}
    html_sections = []
    restart_podnames, state_podnames = set(), set()

    # --- Always Run: Deployment Readiness ---
    deployment_issues = check_deployment_readiness(ns)
    
    # --- Advanced-Only Checks ---
    events = ""
    pvc_issues = []
    if advanced:
        events = run(f"kubectl get events -n {ns} --sort-by=.lastTimestamp")
        pvc_issues = check_pvc_status(ns)

    # --- Error states & Frequent Restarts ---
    # Get detailed JSON output for structured error extraction
    pods_json = run(f"kubectl get pods -n {ns} -o json 2>/dev/null || echo '{{}}'")
    pods_data = {}
    try:
        for pod_obj in json.loads(pods_json).get("items", []):
            pod_name = pod_obj["metadata"]["name"]
            pods_data[pod_name] = pod_obj
    except (json.JSONDecodeError, KeyError):
        pass
    
    for line in pods_lines:
        cols = line.split()
        if not cols or len(cols) < 4:
            continue
        pod, status = cols[0], cols[2]
        try:
            restarts_count = int(cols[3])
        except (ValueError, IndexError):
            restarts_count = 0
        if status in ["CrashLoopBackOff", "ImagePullBackOff", "Error"]:
            # Capture kubectl describe for pods in error states (last 100 lines for brevity)
            describe_output = run(f"kubectl describe pod {pod} -n {ns}")
            describe_truncated = "\n".join(describe_output.splitlines()[-100:])
            
            # Extract structured details from JSON for automation
            pod_json = pods_data.get(pod, {})
            pod_status = pod_json.get("status", {})
            container_statuses = pod_status.get("containerStatuses", [])
            
            # Get container state and reason/message
            reason, message, container_state = status, "", {}
            last_transition_time = ""
            if container_statuses:
                for cs in container_statuses:
                    state = cs.get("state", {})
                    if "waiting" in state:
                        reason = state["waiting"].get("reason", status)
                        message = state["waiting"].get("message", "")
                        container_state = {"waiting": state["waiting"]}
                        break
                    elif "terminated" in state:
                        reason = state["terminated"].get("reason", status)
                        message = state["terminated"].get("message", "")
                        container_state = {"terminated": state["terminated"]}
                        break
            
            # Get last event message from describe output
            last_event = ""
            if "Events:" in describe_output:
                # Find the last Warning or Error event
                events_section = describe_output.split("Events:")[-1]
                for event_line in reversed(events_section.splitlines()):
                    if "Warning" in event_line or "Failed" in event_line:
                        last_event = event_line.strip()
                        break
            
            # Get conditions for timestamp
            conditions = pod_status.get("conditions", [])
            for cond in conditions:
                if cond.get("type") == "Ready" and cond.get("status") == "False":
                    last_transition_time = cond.get("lastTransitionTime", "")
                    break
            
            issues.append({
                "pod": pod,
                "status": status,
                "namespace": ns,
                "describe": describe_truncated,
                # Structured fields for JSON/automation
                "reason": reason,
                "message": message,
                "last_event": last_event,
                "restart_count": restarts_count,
                "container_state": container_state,
                "last_transition_time": last_transition_time
            })
            state_podnames.add(pod)
        if restarts_count >= restart_threshold:
            restarts.append({"pod": pod, "count": restarts_count, "namespace": ns})
            restart_podnames.add(pod)

    all_problem_pods = state_podnames | restart_podnames

    # --- Container Logs / Previous Logs ---
    if include_logs and all_problem_pods:
        for pod in all_problem_pods:
            log1 = run(f"kubectl logs {pod} -n {ns} --tail=20")
            log2 = run(f"kubectl logs {pod} -n {ns} --previous --tail=20")
            pod_logs[pod] = {"current_log": log1, "previous_log": log2}

    # --- Policy Violations & Probe Failures (Advanced Only) ---
    if advanced and events:
        for line in events.splitlines():
            if "PolicyViolation" in line:
                policy_violations.append(line.strip())
            if any(
                x in line
                for x in [
                    "probe failed",
                    "Unhealthy",
                    "Readiness probe failed",
                    "Startup probe failed",
                ]
            ):
                probe_failures.append(line.strip())

    # --- Markdown Output ---
    md = [f"### Namespace: {ns}\n"]
    md.append(f"**Pods:**\n```\n{pods}\n```")
    md.append(f"**Deployments:**\n```\n{deployments}\n```")
    if advanced and events:
        md.append(f"**Events:**\n```\n{events}\n```")
    if advanced and policy_violations:
        md.append("**Policy Violations:**")
        md += policy_violations
    if restarts:
        md.append("**Frequent Restarts:**")
        for r in restarts:
            md.append(f"- Pod `{r['pod']}`: {r['count']} restarts")
    if advanced and probe_failures:
        md.append("**Probe Failures:**")
        md += probe_failures
    if deployment_issues:
        md.append("**⚠️ Deployments/StatefulSets Not Ready:**")
        for issue in deployment_issues:
            md.append(
                f"- {issue['type']}: `{issue['name']}` ({issue['ready']}/{issue['expected']} ready)"
            )
            if issue.get("describe"):
                md.append(f"\n**kubectl describe {issue['type'].lower()} {issue['name']}:**\n```\n{issue['describe']}\n```\n")
    if advanced and pvc_issues:
        md.append("**⚠️ PVC Issues:**")
        for issue in pvc_issues:
            md.append(f"- PVC `{issue['pvc']}`: {issue['status']}")
    if include_logs and pod_logs:
        md.append("**Pod Logs for Problematic Pods:**")
        for pod, logs in pod_logs.items():
            md.append(
                f"**Logs for pod {pod} (last 20 lines):**\n"
                f"```\n{logs['current_log']}\n```"
            )
            md.append(
                f"**Previous container logs for pod {pod} (if any):**\n"
                f"```\n{logs['previous_log']}\n```"
            )
    # Determine if namespace has critical errors (always) or verbose issues (when advanced)
    critical_errors = bool(issues or restarts or deployment_issues)
    verbose_issues = bool(policy_violations or probe_failures or pvc_issues) if advanced else False
    has_errors = critical_errors or verbose_issues
    if errors_only and not has_errors:
        md = []

    # --- HTML Output ---
    log_html = ""
    if include_logs and pod_logs:
        for pod, logs in pod_logs.items():
            log_html += (
                f"<h4>Logs for pod <code>{pod}</code> (last 20 lines):</h4>"
                f"<pre>{logs['current_log']}</pre>"
                f"<h4>Previous container logs for pod <code>{pod}</code> (if any):</h4>"
                f"<pre>{logs['previous_log']}</pre>"
            )
    # Build HTML sections conditionally
    events_html = f"<pre><b>Events:</b>\n{events}</pre>" if advanced and events else ""
    violations_html = "<b>Policy Violations:</b><pre>" + "<br>".join(policy_violations) + "</pre>" if advanced and policy_violations else ""
    restarts_html = "<b>Frequent Restarts:</b><pre>" + "<br>".join([f"{r['pod']}: {r['count']} restarts" for r in restarts]) + "</pre>" if restarts else ""
    probes_html = "<b>Probe Failures:</b><pre>" + "<br>".join(probe_failures) + "</pre>" if advanced and probe_failures else ""
    
    # Build deployments HTML with describe output
    deployments_html = ""
    if deployment_issues:
        deployments_html = "<b>⚠️ Deployments Not Ready:</b>"
        for d in deployment_issues:
            deployments_html += f"<pre>{d['type']}: {d['name']} ({d['ready']}/{d['expected']} ready)</pre>"
            if d.get("describe"):
                deployments_html += f"<details><summary>kubectl describe {d['type'].lower()} {d['name']}</summary><pre>{d['describe']}</pre></details>"
    
    pvc_html = "<b>⚠️ PVC Issues:</b><pre>" + "<br>".join([f"{p['pvc']}: {p['status']}" for p in pvc_issues]) + "</pre>" if advanced and pvc_issues else ""
    
    html_sections.append(
        f"""<details {'open' if has_errors else ''}><summary>"""
        f"""<span style="color:{'red' if has_errors else 'green'}">"""
        f"""Namespace: {ns}</span></summary>
        <pre><b>Deployments:</b>\n{deployments}</pre>
        <pre><b>Pods:</b>\n{pods}</pre>
        {events_html}
        {violations_html}
        {restarts_html}
        {probes_html}
        {deployments_html}
        {pvc_html}
        {log_html if include_logs and pod_logs else ""}
        </details>"""
    )
    # --- Return results for central summary ---
    return {
        "namespace": ns,
        "issues": issues,
        "restarts": restarts,
        "policy_violations": policy_violations,
        "probe_failures": probe_failures,
        "events": events.splitlines()[1:] if len(events.splitlines()) > 1 else [],
        "markdown": md,
        "html": html_sections,
        "pod_logs": pod_logs if include_logs else {},
        "deployment_issues": deployment_issues,
        "pvc_issues": pvc_issues,
    }


# --------------------------------------------------------------------------------------
# AGGREGATION, SUMMARY, REPORTS WRITING
# --------------------------------------------------------------------------------------


# pylint: disable=too-many-locals,too-many-branches,too-many-statements
def main():
    """
    Main execution flow for Kubernetes diagnostics utility.
    
    Workflow:
    1. Parse command-line arguments
    2. Discover all cluster namespaces dynamically
    3. Check ArgoCD applications cluster-wide (if ArgoCD is installed)
    4. Run diagnostics for each namespace (pods, deployments, PVCs, events)
    5. Check node health and job status cluster-wide
    6. Aggregate all findings into summary
    7. Generate outputs:
       - Markdown report (always)
       - HTML report (if --output-html)
       - JSON summary (if --output-json)
    8. Print summary to console
    """
    args = parse_args()
    now = datetime.now()
    ts = now.strftime("%m%d%Y_%H%M%S")
    fname_base = f"diagnostics_full_{ts}"
    all_ns = get_all_namespaces()
    if not all_ns:
        sys.exit("No namespaces found. Check kubectl access / context.")

    # Initialize summary to aggregate all cluster issues
    summary = {
        "pods_w_errors": [],                        # Pods in error/crash states
        "pods_with_many_restarts": [],              # Pods exceeding restart threshold
        "namespaces_with_policy_violations": [],    # Namespaces with policy issues
        "namespaces_with_probe_failures": [],       # Namespaces with probe failures
        "failed_jobs": [],                          # Jobs that failed or incomplete
        "unhealthy_nodes": [],                      # Nodes not in Ready state
        "deployments_not_ready": [],                # Deployments with insufficient replicas
        "pvc_not_bound": [],                        # PVCs not bound to volumes
        "argocd_apps_unhealthy": [],                # ArgoCD apps not healthy/synced
    }
    namespace_reports = []
    html_sections = []

    # --- ArgoCD Applications Check (Cluster-wide) ---
    # Check all ArgoCD applications across namespaces for health/sync issues
    # Runs once cluster-wide, not per-namespace
    argo_issues = check_argocd_apps()
    summary["argocd_apps_unhealthy"].extend(argo_issues)

    # --- Gather Diagnostics for Each Namespace ---
    # Iterate through all discovered namespaces and collect detailed diagnostics
    for ns in all_ns:
        diag = gather_namespace_diagnostics(
            ns, args.restart_threshold, args.errors_only, args.include_logs, args.advanced
        )
        namespace_reports.append(diag)
        html_sections.extend(diag["html"])
        summary["pods_w_errors"].extend(diag["issues"])
        if diag["restarts"]:
            summary["pods_with_many_restarts"].extend(diag["restarts"])
        if diag["policy_violations"]:
            summary["namespaces_with_policy_violations"].append(ns)
        if diag["probe_failures"]:
            summary["namespaces_with_probe_failures"].append(ns)
        if diag["deployment_issues"]:
            summary["deployments_not_ready"].extend(diag["deployment_issues"])
        if diag["pvc_issues"]:
            summary["pvc_not_bound"].extend(diag["pvc_issues"])

    # --- Node Health & Job Failures ---
    nodes = run("kubectl get nodes -o wide")
    for line in nodes.splitlines()[1:]:
        cols = line.split()
        if len(cols) > 1 and cols[1] != "Ready":
            summary["unhealthy_nodes"].append(line.strip())
    jobs = run("kubectl get jobs -A")
    for line in jobs.splitlines()[1:]:
        if "Failed" in line or "0/" in line.split():
            summary["failed_jobs"].append(line.strip())

    # --- Markdown report summary ---
    summary_md = []
    summary_md.append(f"# 🩺 Kubernetes Diagnostics Run - {ts}")
    summary_md.append(f"Namespaces scanned: {len(all_ns)}")
    summary_md.append(f"Nodes: {len(nodes.splitlines())-1}")
    summary_md.append("## 🚩 Issues Summary:")

    def md_issues(header, items, key="pod"):
        if items:
            summary_md.append(f"**{header}:**")
            for item in items:
                if isinstance(item, dict):
                    summary_md.append(
                        f"- {item.get(key, str(item))} in {item.get('namespace','?')}"
                    )
                else:
                    summary_md.append(f"- {item}")
        else:
            summary_md.append(f"**{header}:** _None detected._")

    md_issues("Pods in error state", summary["pods_w_errors"])
    
    # Add kubectl describe output for pods in error states
    if summary["pods_w_errors"]:
        summary_md.append("\n### kubectl describe output for pods in error states:\n")
        for pod_issue in summary["pods_w_errors"]:
            if pod_issue.get("describe"):
                summary_md.append(
                    f"\n**Pod: `{pod_issue['pod']}` (status: {pod_issue['status']}, namespace: `{pod_issue['namespace']}`)**"
                )
                summary_md.append(f"```\n{pod_issue['describe']}\n```\n")
    
    md_issues("Pods with frequent restarts", summary["pods_with_many_restarts"])
    summary_md.append(
        f"Policy violations in "
        f"{len(summary['namespaces_with_policy_violations'])} namespaces"
    )
    summary_md.append(
        f"Probe failures in "
        f"{len(summary['namespaces_with_probe_failures'])} namespaces"
    )
    md_issues(
        "Unhealthy nodes",
        summary["unhealthy_nodes"],
        key=0 if summary["unhealthy_nodes"] else "pod",
    )
    md_issues(
        "Failed Jobs",
        summary["failed_jobs"],
        key=0 if summary["failed_jobs"] else "pod",
    )

    # --- Advanced Diagnostics Summary ---
    summary_md.append("\n## 🔧 Deployment & Resource Status:")
    if summary["deployments_not_ready"]:
        summary_md.append("**⚠️ Deployments/StatefulSets Not Ready:**")
        for d in summary["deployments_not_ready"]:
            summary_md.append(
                f"- {d['type']}: `{d['name']}` in `{d['namespace']}` "
                f"({d['ready']}/{d['expected']} ready)"
            )
    else:
        summary_md.append("**Deployments/StatefulSets:** _All ready._")

    if summary["pvc_not_bound"]:
        summary_md.append("**⚠️ PVCs Not Bound:**")
        for p in summary["pvc_not_bound"]:
            summary_md.append(f"- `{p['pvc']}` in `{p['namespace']}`: {p['status']}")
    else:
        summary_md.append("**PVCs:** _All bound._")

    if summary["argocd_apps_unhealthy"]:
        summary_md.append("**⚠️ ArgoCD Applications Not Healthy:**")
        for app in summary["argocd_apps_unhealthy"]:
            summary_md.append(
                f"- `{app['name']}` in `{app['namespace']}`: "
                f"Health={app['health']}, Sync={app['sync']}"
            )
    else:
        summary_md.append("**ArgoCD Applications:** _All healthy and synced._")

    # --- ArgoCD Applications Details Section ---
    if summary["argocd_apps_unhealthy"]:
        summary_md.append("\n## 🔄 ArgoCD Applications Details:")
        summary_md.append("\n| Application | Namespace | Health | Sync | Message |")
        summary_md.append("|-------------|-----------|--------|------|---------|")
        for app in summary["argocd_apps_unhealthy"]:
            # Don't truncate in markdown - full details needed for debugging
            msg = app.get('message', 'N/A')
            summary_md.append(
                f"| `{app['name']}` | `{app['namespace']}` | "
                f"{app['health']} | {app['sync']} | {msg} |"
            )
        
        # Add kubectl describe output for each unhealthy application
        summary_md.append("\n### kubectl describe output for unhealthy applications:\n")
        for app in summary["argocd_apps_unhealthy"]:
            if app.get("describe"):
                summary_md.append(f"\n**Application: `{app['name']}` (namespace: `{app['namespace']}`)**")
                summary_md.append(f"```\n{app['describe']}\n```\n")
    
    summary_md.append("---\n")

    # --- Write Markdown report output ---
    with open(fname_base + ".md", "w", encoding="utf-8") as f:
        f.write("\n".join(summary_md))
        for report in namespace_reports:
            if report["markdown"]:
                f.write("\n" + "\n".join(report["markdown"]))
    
    # Print concise summary to console (full details in report files)
    print(f"\n{'='*80}")
    print(f"Kubernetes Diagnostics Report Generated: {ts}")
    print(f"{'='*80}")
    print(f"Namespaces scanned: {len(all_ns)}")
    print(f"Nodes: {len(nodes.splitlines())-1}")
    print(f"Pods with errors: {len(summary['pods_w_errors'])}")
    print(f"Pods with frequent restarts: {len(summary['pods_with_many_restarts'])}")
    print(f"Deployments not ready: {len(summary['deployments_not_ready'])}")
    print(f"ArgoCD apps unhealthy: {len(summary['argocd_apps_unhealthy'])}")
    print(f"Jobs incomplete: {len(summary['failed_jobs'])}")
    print(f"\nReports saved:")
    print(f"  - {fname_base}.md (detailed markdown)")
    if args.output_html:
        print(f"  - {fname_base}.html (shareable HTML)")
    if args.output_json:
        print(f"  - {fname_base}.json (machine-readable)")
    print(f"{'='*80}\n")

    # --- Write JSON output if requested ---
    if args.output_json:
        # Helper function to remove verbose describe field from items
        def strip_describe(item):
            """Remove describe field, keep structured data for automation."""
            if isinstance(item, dict):
                return {k: v for k, v in item.items() if k != "describe"}
            return item
        
        # Generate recommendations based on error patterns
        recommendations = []
        for pod_err in summary["pods_w_errors"]:
            issue_desc = f"{pod_err['status']} in {pod_err['namespace']}/{pod_err['pod']}"
            if "ImagePullBackOff" in pod_err.get("status", ""):
                recommendations.append({
                    "issue": issue_desc,
                    "severity": "high",
                    "suggested_action": "Check image name, registry credentials, and network connectivity to registry"
                })
            elif "CrashLoopBackOff" in pod_err.get("status", ""):
                recommendations.append({
                    "issue": issue_desc,
                    "severity": "high",
                    "suggested_action": "Check pod logs for application errors, verify environment variables and config"
                })
            elif "Error" in pod_err.get("status", ""):
                recommendations.append({
                    "issue": issue_desc,
                    "severity": "medium",
                    "suggested_action": "Review pod events and logs for root cause"
                })
        
        for deploy in summary["deployments_not_ready"]:
            if deploy.get("reason") == "ProgressDeadlineExceeded":
                recommendations.append({
                    "issue": f"Deployment {deploy['namespace']}/{deploy['name']} rollout stuck",
                    "severity": "high",
                    "suggested_action": "Check pod events, resource quotas, and image availability"
                })
        
        json_obj = {
            "ts": ts,
            "has_errors": len(summary["pods_w_errors"]) > 0 or len(summary["deployments_not_ready"]) > 0,
            "summary": {
                # Strip describe fields from summary items for cleaner JSON
                "pods_w_errors": [strip_describe(p) for p in summary["pods_w_errors"]],
                "pods_with_many_restarts": summary["pods_with_many_restarts"],
                "namespaces_with_policy_violations": summary["namespaces_with_policy_violations"],
                "namespaces_with_probe_failures": summary["namespaces_with_probe_failures"],
                "failed_jobs": summary["failed_jobs"],
                "unhealthy_nodes": summary["unhealthy_nodes"],
                "deployments_not_ready": [strip_describe(d) for d in summary["deployments_not_ready"]],
                "pvc_not_bound": summary["pvc_not_bound"],
                "argocd_apps_unhealthy": [strip_describe(a) for a in summary["argocd_apps_unhealthy"]],
            },
            "recommendations": recommendations,
            "namespace_reports": [
                {
                    "namespace": n["namespace"],
                    "issues": [strip_describe(i) for i in n["issues"]],
                    "restarts": n["restarts"],
                    "policy_violations": n["policy_violations"],
                    "probe_failures": n["probe_failures"],
                    "deployment_issues": [strip_describe(d) for d in n["deployment_issues"]],
                    "pvc_issues": n["pvc_issues"],
                }
                for n in namespace_reports
            ],
        }
        with open(fname_base + ".json", "w", encoding="utf-8") as f:
            json.dump(json_obj, f, indent=2)

    # --- Write HTML report if requested ---
    if args.output_html:
        with open(fname_base + ".html", "w", encoding="utf-8") as f:
            f.write(
                f"<!DOCTYPE html><html><head><meta charset='utf-8'/>"
                f"<title>K8s Diagnostics Report - {ts}</title>\n"
            )
            f.write(
                "<style>body{font-family:sans-serif; background:#f8f8fc; "
                "color:#24292f;} h1{color:#0366d6} summary{font-weight: bold; "
                "font-size: 1.1em;} details[open] summary{color:#d73a49;} "
                "pre { background:#f2f4fa; padding:.5em 1em; "
                "border-radius:5px;overflow-x:auto; }</style></head><body>\n"
            )
            f.write(
                f"<h1>Kubernetes Diagnostic Report ({ts})</h1>\n"
                "<section>\n<h2>Summary</h2>\n<ul>"
            )
            f.write(f"<li><b>Namespaces scanned:</b> {len(all_ns)}</li>\n")
            f.write(f"<li><b>Nodes:</b> {len(nodes.splitlines())-1}</li>\n")
            f.write(
                f"<li><b>Pods with errors:</b> "
                f"{len(summary['pods_w_errors'])}</li>\n"
            )
            f.write(
                f"<li><b>Pods with frequent restarts:</b> "
                f"{len(summary['pods_with_many_restarts'])}</li>\n"
            )
            f.write(
                f"<li><b>Namespaces with policy violations:</b> "
                f"{len(summary['namespaces_with_policy_violations'])}</li>\n"
            )
            f.write(f"<li><b>Job failures:</b> {len(summary['failed_jobs'])}</li>\n")
            f.write(
                f"<li><b>Unhealthy nodes:</b> {len(summary['unhealthy_nodes'])}</li>\n"
            )
            f.write(
                f"<li><b>⚠️ Deployments not ready:</b> "
                f"{len(summary['deployments_not_ready'])}</li>\n"
            )
            f.write(
                f"<li><b>⚠️ PVCs not bound:</b> "
                f"{len(summary['pvc_not_bound'])}</li>\n"
            )
            f.write(
                f"<li><b>⚠️ ArgoCD apps unhealthy:</b> "
                f"{len(summary['argocd_apps_unhealthy'])}</li>\n"
            )
            f.write("</ul>\n</section>\n")
            
            # Pods in Error States with kubectl describe
            if summary["pods_w_errors"]:
                f.write("<section>\n<h2>🚨 Pods in Error States</h2>\n")
                f.write("<h3>kubectl describe output for pods in error states</h3>\n")
                for pod_issue in summary["pods_w_errors"]:
                    if pod_issue.get("describe"):
                        f.write(
                            f"<details><summary><b>Pod: <code>{pod_issue['pod']}</code> "
                            f"(status: {pod_issue['status']}, namespace: <code>{pod_issue['namespace']}</code>)</b></summary>"
                            f"<pre>{pod_issue['describe']}</pre></details>\n"
                        )
                f.write("</section>\n")
            
            # ArgoCD Applications Details Section
            if summary["argocd_apps_unhealthy"]:
                f.write("<section>\n<h2>🔄 ArgoCD Applications Details</h2>\n")
                f.write("<table border='1' cellpadding='8' cellspacing='0' style='border-collapse:collapse;'>\n")
                f.write("<tr style='background:#0366d6;color:white;'>")
                f.write("<th>Application</th><th>Namespace</th><th>Health</th><th>Sync</th><th>Message</th></tr>\n")
                for app in summary["argocd_apps_unhealthy"]:
                    health_color = "red" if app['health'] not in ["Healthy", "Progressing"] else "orange"
                    sync_color = "red" if app['sync'] != "Synced" else "green"
                    # Don't truncate - show full error message in HTML for debugging
                    msg = app.get('message', 'N/A')
                    f.write(
                        f"<tr><td><code>{app['name']}</code></td>"
                        f"<td><code>{app['namespace']}</code></td>"
                        f"<td style='color:{health_color};font-weight:bold;'>{app['health']}</td>"
                        f"<td style='color:{sync_color};font-weight:bold;'>{app['sync']}</td>"
                        f"<td>{msg}</td></tr>\n"
                    )
                f.write("</table>\n")
                
                # Add kubectl describe output for each unhealthy application
                f.write("<h3>kubectl describe output for unhealthy applications</h3>\n")
                for app in summary["argocd_apps_unhealthy"]:
                    if app.get("describe"):
                        f.write(
                            f"<details><summary><b>Application: <code>{app['name']}</code> "
                            f"(namespace: <code>{app['namespace']}</code>)</b></summary>"
                            f"<pre>{app['describe']}</pre></details>\n"
                        )
                f.write("</section>\n")
            
            for html in html_sections:
                f.write(html)
            f.write("\n</body></html>")


if __name__ == "__main__":
    main()