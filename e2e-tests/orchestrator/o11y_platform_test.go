// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/open-edge-platform/edge-manageability-framework/e2e-tests/orchestrator/helpers"
)

const (
	orchObs              = "orchestrator-observability"
	platformObsNamespace = "orch-platform"
	mimirGatewayPort     = 12000
	lokiWritePort        = 12001
	lokiReadPort         = 12002
	metricsEndpoint      = "/prometheus/api/v1/query"
	logsEndpoint         = "/loki/api/v1/query_range"
	logWriteEndpoint     = "/loki/api/v1/push"
	orchSystem           = "orchestrator-system"
)

var (
	upMetrics      = []string{"up"}
	istioMetrics   = []string{"istio_requests_total"}
	kubeletMetrics = []string{
		"k8s_node_allocatable_cpu",
		"k8s_node_allocatable_memory",
		"k8s_node_cpu_utilization",
		"k8s_node_filesystem_capacity",
		"k8s_node_filesystem_usage",
		"k8s_node_memory_available",
		"k8s_node_memory_usage",
		"k8s_node_memory_working_set",
		"k8s_node_network_errors",
		"k8s_node_network_io",
		"k8s_pod_cpu_utilization",
		"k8s_pod_memory_usage",
		"k8s_pod_memory_working_set",
		"container_cpu_utilization",
		"container_memory_working_set",
	}
	kubeMetrics = []string{
		"kube_configmap_info",
		"kube_endpoint_info",
		"kube_ingress_info",
		"kube_node_info",
		"kube_node_status_condition",
		"kube_persistentvolumeclaim_info",
		"kube_pod_container_info",
		"kube_pod_container_resource_limits",
		"kube_pod_container_resource_requests",
		"kube_pod_container_status_running",
		"kube_pod_info",
		"kube_pod_status_phase",
		"kube_secret_info",
		"kube_service_info",
		"kube_statefulset_replicas",
	}
	controllerMetrics = []string{"controller_runtime_reconcile_errors_total", "controller_runtime_reconcile_total"}
	admMetrics        = []string{"adm_deployment_status"}
	otelMetrics       = []string{
		"otelcol_exporter_queue_capacity",
		"otelcol_exporter_queue_size",
		"otelcol_exporter_send_failed_log_records",
		"otelcol_exporter_send_failed_metric_points",
		"otelcol_exporter_sent_log_records",
		"otelcol_exporter_sent_metric_points",
		"otelcol_otelsvc_k8s_pod_added",
		"otelcol_otelsvc_k8s_pod_deleted",
		"otelcol_otelsvc_k8s_pod_updated",
		"otelcol_process_cpu_seconds",
		"otelcol_process_memory_rss",
		"otelcol_processor_accepted_log_records",
		"otelcol_processor_accepted_metric_points",
		"otelcol_processor_batch_batch_send_size_bucket",
		"otelcol_processor_batch_batch_send_size_count",
		"otelcol_processor_batch_batch_send_size_sum",
		"otelcol_processor_batch_batch_size_trigger_send",
		"otelcol_processor_batch_metadata_cardinality",
		"otelcol_processor_batch_timeout_trigger_send",
		"otelcol_process_runtime_heap_alloc_bytes",
		"otelcol_process_runtime_total_sys_memory_bytes",
		"otelcol_process_uptime",
		"otelcol_receiver_accepted_log_records",
		"otelcol_receiver_accepted_metric_points",
		"otelcol_receiver_refused_log_records",
		"otelcol_receiver_refused_metric_points",
	}
	processMetrics   = []string{"process_cpu_seconds_total", "process_resident_memory_bytes"}
	traefikMetrics   = []string{"traefik_service_request_duration_seconds_bucket", "traefik_service_requests_total"}
	workqueueMetrics = []string{
		"workqueue_adds_total",
		"workqueue_queue_duration_seconds_bucket",
		"workqueue_retries_total",
		"workqueue_work_duration_seconds_bucket",
	}
	logsServiceNames = []string{
		"alerting-monitor",
		"alertmanager",
		"alertmanager-configmap-reload",
		"compactor",
		"config-reloader",
		"distributor",
		"etcd",
		"gateway",
		"grafana",
		"grafana-proxy",
		"grafana-sc-dashboard",
		"ingester",
		"istio-init",
		"istio-proxy",
		"keycloak",
		"kube-apiserver",
		"kube-controller-manager",
		"kube-prometheus-stack",
		"loki",
		"loki-sc-rules",
		"metrics-exporter",
		"nginx",
		"observability-tenant-controller",
		"opentelemetry-collector",
		"otel-collector",
		"prometheus",
		"querier",
		"query-frontend",
		"ruler",
		"traefik",
		"vault",
	}
)

var _ = Describe("Orchestrator Observability Test:", Ordered, Label(orchObs), func() {
	var (
		cli              *http.Client
		kubecmdMimir     *exec.Cmd
		kubecmdLokiRead  *exec.Cmd
		kubecmdLokiWrite *exec.Cmd
		mimirURL         string
		lokiReadURL      string
		lokiWriteURL     string
		metricsAddr      string
		logsAddr         string
	)

	BeforeAll(func() {
		mimirURL = fmt.Sprintf("http://localhost:%v", mimirGatewayPort)
		metricsAddr = mimirURL + metricsEndpoint
		argsMimir := []string{
			"port-forward",
			"svc/orchestrator-observability-mimir-gateway",
			"-n",
			platformObsNamespace,
			strconv.Itoa(mimirGatewayPort) + ":8181",
		}
		kubecmdMimir = exec.Command("kubectl", argsMimir...)
		err := kubecmdMimir.Start()
		Expect(err).ToNot(HaveOccurred())

		lokiReadURL = fmt.Sprintf("http://localhost:%v", lokiReadPort)
		logsAddr = lokiReadURL + logsEndpoint
		argsLokiRead := []string{
			"port-forward",
			"svc/loki-read",
			"-n",
			platformObsNamespace,
			strconv.Itoa(lokiReadPort) + ":3100",
		}
		kubecmdLokiRead = exec.Command("kubectl", argsLokiRead...)
		err = kubecmdLokiRead.Start()
		Expect(err).ToNot(HaveOccurred())

		lokiWriteURL = fmt.Sprintf("http://localhost:%v", lokiWritePort)
		argsLokiWrite := []string{
			"port-forward",
			"svc/loki-write",
			"-n",
			platformObsNamespace,
			strconv.Itoa(lokiWritePort) + ":3100",
		}
		kubecmdLokiWrite = exec.Command("kubectl", argsLokiWrite...)
		err = kubecmdLokiWrite.Start()
		Expect(err).ToNot(HaveOccurred())

		time.Sleep(2 * time.Second)
	})

	BeforeEach(func() {
		cli = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
	})

	Context("Platform metrics", func() {
		It("Mimir gateway should be in running state", func() {
			resp, err := helpers.MakeRequest(http.MethodGet, fmt.Sprintf("%v/ready", mimirURL), nil, cli, nil)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("Up metric should be present in orchestrator", func() {
			metricsNotFound := make(map[string]struct{})
			for _, metric := range upMetrics {
				found, err := helpers.CheckMetric(cli, metricsAddr, metric, orchSystem)
				Expect(err).ToNot(HaveOccurred())
				if !found {
					metricsNotFound[metric] = struct{}{}
				}
			}
			Expect(metricsNotFound).To(BeEmpty(), "%v metrics not found", metricsNotFound)
		})

		It("Istio metrics should be present in orchestrator", func() {
			metricsNotFound := make(map[string]struct{})
			for _, metric := range istioMetrics {
				found, err := helpers.CheckMetric(cli, metricsAddr, metric, orchSystem)
				Expect(err).ToNot(HaveOccurred())
				if !found {
					metricsNotFound[metric] = struct{}{}
				}
			}
			Expect(metricsNotFound).To(BeEmpty(), "%v metrics not found", metricsNotFound)
		})

		It("Kubelet metrics should be present in orchestrator", func() {
			metricsNotFound := make(map[string]struct{})
			for _, metric := range kubeletMetrics {
				found, err := helpers.CheckMetric(cli, metricsAddr, metric, orchSystem)
				Expect(err).ToNot(HaveOccurred())
				if !found {
					metricsNotFound[metric] = struct{}{}
				}
			}
			Expect(metricsNotFound).To(BeEmpty(), "%v metrics not found", metricsNotFound)
		})

		It("Kubernetes metrics should be present in orchestrator", func() {
			metricsNotFound := make(map[string]struct{})
			for _, metric := range kubeMetrics {
				found, err := helpers.CheckMetric(cli, metricsAddr, metric, orchSystem)
				Expect(err).ToNot(HaveOccurred())
				if !found {
					metricsNotFound[metric] = struct{}{}
				}
			}
			Expect(metricsNotFound).To(BeEmpty(), "%v metrics not found", metricsNotFound)
		})

		It("Controller metrics should be present in orchestrator", func() {
			metricsNotFound := make(map[string]struct{})
			for _, metric := range controllerMetrics {
				found, err := helpers.CheckMetric(cli, metricsAddr, metric, orchSystem)
				Expect(err).ToNot(HaveOccurred())
				if !found {
					metricsNotFound[metric] = struct{}{}
				}
			}
			Expect(metricsNotFound).To(BeEmpty(), "%v metrics not found", metricsNotFound)
		})

		It("ADM metrics should be present in orchestrator", func() {
			metricsNotFound := make(map[string]struct{})
			for _, metric := range admMetrics {
				found, err := helpers.CheckMetric(cli, metricsAddr, metric, orchSystem)
				Expect(err).ToNot(HaveOccurred())
				if !found {
					metricsNotFound[metric] = struct{}{}
				}
			}
			Expect(metricsNotFound).To(BeEmpty(), "%v metrics not found", metricsNotFound)
		})

		It("OTEL metrics should be present in orchestrator", func() {
			metricsNotFound := make(map[string]struct{})
			for _, metric := range otelMetrics {
				found, err := helpers.CheckMetric(cli, metricsAddr, metric, orchSystem)
				Expect(err).ToNot(HaveOccurred())
				if !found {
					metricsNotFound[metric] = struct{}{}
				}
			}
			Expect(metricsNotFound).To(BeEmpty(), "%v metrics not found", metricsNotFound)
		})

		It("Process metrics should be present in orchestrator", func() {
			metricsNotFound := make(map[string]struct{})
			for _, metric := range processMetrics {
				found, err := helpers.CheckMetric(cli, metricsAddr, metric, orchSystem)
				Expect(err).ToNot(HaveOccurred())
				if !found {
					metricsNotFound[metric] = struct{}{}
				}
			}
			Expect(metricsNotFound).To(BeEmpty(), "%v metrics not found", metricsNotFound)
		})

		It("Traefik metrics should be present in orchestrator", func() {
			metricsNotFound := make(map[string]struct{})
			for _, metric := range traefikMetrics {
				found, err := helpers.CheckMetric(cli, metricsAddr, metric, orchSystem)
				Expect(err).ToNot(HaveOccurred())
				if !found {
					metricsNotFound[metric] = struct{}{}
				}
			}
			Expect(metricsNotFound).To(BeEmpty(), "%v metrics not found", metricsNotFound)
		})

		It("Workqueue metrics should be present in orchestrator", func() {
			metricsNotFound := make(map[string]struct{})
			for _, metric := range workqueueMetrics {
				found, err := helpers.CheckMetric(cli, metricsAddr, metric, orchSystem)
				Expect(err).ToNot(HaveOccurred())
				if !found {
					metricsNotFound[metric] = struct{}{}
				}
			}
			Expect(metricsNotFound).To(BeEmpty(), "%v metrics not found", metricsNotFound)
		})
	})

	Context("Platform logs", func() {
		It("Loki write should be in running state", func() {
			resp, err := helpers.MakeRequest(http.MethodGet, fmt.Sprintf("%v/ready", lokiWriteURL), nil, cli, nil)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("Loki read should be in running state", func() {
			resp, err := helpers.MakeRequest(http.MethodGet, fmt.Sprintf("%v/ready", lokiReadURL), nil, cli, nil)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("Loki push api should be working correctly", func() {
			now := time.Now().Unix()
			body := fmt.Sprintf(`{
				"streams": [
					{
					"stream": {
						"app": "validation"
					},
					"values": [
						["%v000000000", "log message"]
					]
					}
				]
				}`, now)

			headers := make(http.Header)
			headers.Add("X-Scope-OrgID", orchSystem)
			headers.Add("Content-Type", "application/json")

			resp, err := helpers.MakeRequest(http.MethodPost, fmt.Sprintf("%v%v", lokiWriteURL, logWriteEndpoint), []byte(body), cli, headers)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
		})

		It("Loki query api should be working correctly", func() {
			query := "{app=\"validation\"}"
			logs, err := helpers.GetLogs(cli, logsAddr, query, since1h, orchSystem)
			Expect(err).ToNot(HaveOccurred())

			Expect(logs.Data.Result).ToNot(BeEmpty())
			Expect(logs.Data.Result[0].Stream.ServiceName).Should(Equal("validation"))
		})

		It("Audit logs must be present in orchestrator", func() {
			query := "{k8s_namespace_name=~\".+\"} |~ \"\\\"component\\\":\\\"Audit\\\"\" | json"
			logs, err := helpers.GetLogs(cli, logsAddr, query, since1h, orchSystem)
			Expect(err).ToNot(HaveOccurred())

			Expect(logs.Data.Result).ToNot(BeEmpty())
		})

		It("System logs must be present in orchestrator", func() {
			Eventually(func() (map[string]struct{}, error) {
				logsNotFound := make(map[string]struct{})
				for _, name := range logsServiceNames {
					query := fmt.Sprintf("{service_name=~\"%v\"}", name)
					logs, err := helpers.GetLogs(cli, logsAddr, query, since1h, orchSystem)
					if err != nil {
						return logsNotFound, err
					}

					if len(logs.Data.Result) == 0 {
						logsNotFound[name] = struct{}{}
					}
				}
				return logsNotFound, nil
			}, 4*time.Minute, 10*time.Second).Should(BeEmpty(), "eventually expected logs should be present")
		})
	})

	AfterAll(func() {
		err := kubecmdMimir.Process.Kill()
		Expect(err).ToNot(HaveOccurred())

		err = kubecmdLokiRead.Process.Kill()
		Expect(err).ToNot(HaveOccurred())

		err = kubecmdLokiWrite.Process.Kill()
		Expect(err).ToNot(HaveOccurred())
	})
})
