// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/open-edge-platform/edge-manageability-framework/e2e-tests/orchestrator/helpers"
	util "github.com/open-edge-platform/edge-manageability-framework/mage"
)

const (
	victoriaMetricsURL     = "http://localhost:8428"
	httpAuthUsername       = "sre"
	sreNamespace           = "orch-sre"
	destinationService     = "svc/sre-exporter-destination"
	orchMetricQueryTimeout = 2 * time.Minute
	enicMetricQueryTimeout = 6 * time.Minute
)

var (
	orchMetrics = []string{
		// Exported Orchestrator Metrics
		"orch_IstioCollector_istio_requests",
		"orch_NodeCollector_cpu_total_cores",
		"orch_NodeCollector_cpu_used_cores",
		"orch_NodeCollector_memory_total_bytes",
		"orch_NodeCollector_memory_available_bytes",
		"orch_api_requests_all",
		"orch_api_request_latency_seconds_all",
		"orch_vault_monitor_vault_status",
	}

	edgenodeMetrics = []string{
		// Exported Edge Node Metrics
		"orch_edgenode_env_temp",
		"orch_edgenode_mem_used_percent",
		"orch_edgenode_disk_used_percent",
		"orch_edgenode_cpu_idle_percent",
	}

	diagnosticMetrics = []string{
		// Exported Diagnostic Metrics
		"orch_IstioCollector",
		"orch_NodeCollector",
		"orch_api",
		"orch_edgenode_env",
		"orch_edgenode_mem",
		"orch_edgenode_disk",
		"orch_edgenode_cpu",
		"orch_vault_status",
	}
)

var _ = Describe("Observability SRE Exporter Test:", Label("sre-observability"), Ordered, func() {
	var (
		client           *http.Client
		kubecmdVM        *exec.Cmd
		user             string
		pass             string
		httpAuthPassword string
		err              error
	)

	BeforeAll(func() {
		vmArgs := []string{
			"port-forward",
			destinationService,
			"-n",
			sreNamespace,
			"8428:8428",
		}
		kubecmdVM = exec.Command("kubectl", vmArgs...)
		Expect(kubecmdVM.Start()).To(Succeed())

		By("Port forwarding to VictoriaMetrics service has been started.")
		time.Sleep(2 * time.Second)

		httpAuthPassword, err = util.GetDefaultOrchPassword()
		Expect(err).ToNot(HaveOccurred())
	})

	BeforeEach(func() {
		client = &http.Client{
			Transport: &http.Transport{},
		}

		user = httpAuthUsername
		pass = httpAuthPassword
	})

	verifyMetric := func(metric string) (string, int) {
		By(fmt.Sprintf("Checking metric: %s", metric))
		req, err := http.NewRequest(http.MethodGet,
			fmt.Sprintf("%s/api/v1/query?query=topk_last(1,%s)", victoriaMetricsURL, metric), nil)
		Expect(err).ToNot(HaveOccurred())

		req.SetBasicAuth(user, pass)

		resp, err := client.Do(req)
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", resp.StatusCode
		}

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		Expect(err).ToNot(HaveOccurred())

		results := result["data"].(map[string]interface{})["result"].([]interface{})
		if len(results) == 0 {
			return "", resp.StatusCode
		}

		value := results[0].(map[string]interface{})["value"].([]interface{})[1].(string)
		return value, resp.StatusCode
	}

	expectNotEmpty := func(metrics []string) error {
		nullMetrics := make([]string, 0, len(metrics))
		for _, metric := range metrics {
			value, status := verifyMetric(metric)
			if status != http.StatusOK {
				return fmt.Errorf("query for metric %s returned status %d", metric, status)
			}
			if value == "" {
				nullMetrics = append(nullMetrics, metric)
			}
		}
		if len(nullMetrics) > 0 {
			return fmt.Errorf("metrics %q returned null values", nullMetrics)
		}
		return nil
	}

	Context("When ENIC is not deployed", func() {
		It("exported orchestrator metrics return non-null values", func() {
			Eventually(expectNotEmpty, orchMetricQueryTimeout, 10*time.Second).WithArguments(orchMetrics).Should(Succeed())
		})

		It("exported edgenode metrics return null values", func() {
			for _, metric := range edgenodeMetrics {
				value, status := verifyMetric(metric)
				Expect(status).To(Equal(http.StatusOK))
				Expect(value).To(BeEmpty(), fmt.Sprintf("Metric %s did not return null value", metric))
			}
		})

		It("wrong password prevents from capturing metrics, resulting 401 response code", func() {
			pass = "invalid-password"
			metric := "orch_vault_monitor_vault_status"
			_, status := verifyMetric(metric)
			Expect(status).To(Equal(http.StatusUnauthorized))
		})
	})

	Context("When ENIC is deployed", Label(helpers.LabelEnic), func() {
		It("diagnostic metric postfixed _up returns 1", func() {
			for _, metric := range diagnosticMetrics {
				value, status := verifyMetric(metric + "_up")
				Expect(status).To(Equal(http.StatusOK))
				Expect(value).To(Equal("1"), fmt.Sprintf("Metric %s_up did not return 1", metric))
			}
		})

		It("diagnostic metric postfixed _warnings returns 0", func() {
			for _, metric := range diagnosticMetrics {
				value, status := verifyMetric(metric + "_warnings")
				Expect(status).To(Equal(http.StatusOK))
				Expect(value).To(Equal("0"), fmt.Sprintf("Metric %s_warnings did not return 0", metric))
			}
		})

		It("exported orchestrator metrics return non-null values", func() {
			Eventually(expectNotEmpty, orchMetricQueryTimeout, 10*time.Second).WithArguments(orchMetrics).Should(Succeed())
		})

		It("exported edgenode metrics return non-null values", func() {
			Eventually(expectNotEmpty, enicMetricQueryTimeout, 10*time.Second).WithArguments(edgenodeMetrics).Should(Succeed())
		})

		It("diagnostic metric postfixed _up are not available when password is incorrect", func() {
			pass = "invalid-password"
			for _, metric := range diagnosticMetrics {
				_, status := verifyMetric(metric + "_up")
				Expect(status).To(Equal(http.StatusUnauthorized))
			}
		})
	})

	AfterAll(func() {
		err := kubecmdVM.Process.Kill()
		Expect(err).ToNot(HaveOccurred())
	})
})
