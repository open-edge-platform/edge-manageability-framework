// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package orchestrator_test

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/open-edge-platform/edge-manageability-framework/e2e-tests/orchestrator/helpers"
	util "github.com/open-edge-platform/edge-manageability-framework/mage"
)

const (
	edgenodeObs        = "edgenode-observability"
	enObsNamespace     = "orch-infra"
	enMimirGatewayPort = 14000
	enLokiWritePort    = 14001
	enLokiReadPort     = 14002
	enMetricsEndpoint  = "/prometheus/api/v1/query"
	enLogsEndpoint     = "/loki/api/v1/query_range"
	enLogWriteEndpoint = "/loki/api/v1/push"
	since1h            = "1h"
	since3h            = "3h"
	// Org and project names must be the same as in step in CI.
	enOrgName     = "sample-org"
	enProjectName = "sample-project"
)

var enMetrics = []string{
	"cpu_usage_idle",
	"cpu_usage_system",
	"cpu_usage_user",
	"diskio_read_bytes",
	"diskio_write_bytes",
	"disk_used_percent",
	"mem_available",
	"mem_buffered",
	"mem_cached",
	"mem_free",
	"mem_total",
	"mem_used",
	"mem_used_percent",
	"net_bytes_recv",
	"net_bytes_sent",
}

var enAgents = []string{
	"ClusterAgent",
	"HardwareAgent",
	"NodeAgent",
	"NodeAgent",
	"PlatformUpdateAgent",
	"Platform_Telemetry_Agent",
}

// Deployment of edge node (eg. ENiC) is necessary for these tests.
var _ = Describe("Edgenode Observability Test:", Ordered, Label(edgenodeObs), func() {
	var (
		cli              *http.Client
		kubecmdMimir     *exec.Cmd
		kubecmdLokiRead  *exec.Cmd
		kubecmdLokiWrite *exec.Cmd
		mimirURL         string
		metricsAddr      string
		projectID        string
		lokiReadURL      string
		lokiWriteURL     string
		logsAddr         string
	)

	BeforeAll(func() {
		mimirURL = fmt.Sprintf("http://localhost:%v", enMimirGatewayPort)
		metricsAddr = mimirURL + enMetricsEndpoint
		mimirArgs := []string{
			"port-forward",
			"svc/edgenode-observability-mimir-gateway",
			"-n",
			enObsNamespace,
			strconv.Itoa(enMimirGatewayPort) + ":8181",
		}
		kubecmdMimir = exec.Command("kubectl", mimirArgs...)
		err := kubecmdMimir.Start()
		Expect(err).ToNot(HaveOccurred())

		lokiReadURL = fmt.Sprintf("http://localhost:%v", enLokiReadPort)
		logsAddr = lokiReadURL + enLogsEndpoint
		argsLokiRead := []string{
			"port-forward",
			"svc/loki-read",
			"-n",
			enObsNamespace,
			strconv.Itoa(enLokiReadPort) + ":3100",
		}
		kubecmdLokiRead = exec.Command("kubectl", argsLokiRead...)
		err = kubecmdLokiRead.Start()
		Expect(err).ToNot(HaveOccurred())

		lokiWriteURL = fmt.Sprintf("http://localhost:%v", enLokiWritePort)
		argsLokiWrite := []string{
			"port-forward",
			"svc/loki-write",
			"-n",
			enObsNamespace,
			strconv.Itoa(enLokiWritePort) + ":3100",
		}
		kubecmdLokiWrite = exec.Command("kubectl", argsLokiWrite...)
		err = kubecmdLokiWrite.Start()
		Expect(err).ToNot(HaveOccurred())

		projectID, err = util.GetProjectId(context.Background(), enOrgName, enProjectName)
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

	Context("Edgenode metrics", func() {
		It("Mimir gateway should be in running state", func() {
			resp, err := helpers.MakeRequest(http.MethodGet, fmt.Sprintf("%v/ready", mimirURL), nil, cli, nil)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("Edgenode metrics must be present in mimir", func() {
			for _, metric := range enMetrics {
				found, err := helpers.CheckMetric(cli, metricsAddr, metric, projectID)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue(), "%s metric not found", metric)
			}
		})
	})

	Context("Edgenode logs", func() {
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
			headers.Add("X-Scope-OrgID", projectID)
			headers.Add("Content-Type", "application/json")

			resp, err := helpers.MakeRequest(http.MethodPost, fmt.Sprintf("%v%v", lokiWriteURL, logWriteEndpoint), []byte(body), cli, headers)
			Expect(err).ToNot(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
		})

		It("Loki query api should be working correctly", func() {
			headers := make(http.Header)
			headers.Add("X-Scope-OrgID", projectID)
			query := "{app=\"validation\"}"
			logs, err := helpers.GetLogs(cli, logsAddr, query, since1h, projectID)
			Expect(err).ToNot(HaveOccurred())

			Expect(logs.Data.Result).ToNot(BeEmpty())
			Expect(logs.Data.Result[0].Stream.ServiceName).Should(Equal("validation"))
		})

		It("Logs should from agents should be present in edgenode loki", func() {
			logsNotFound := make(map[string]struct{})

			for _, name := range enAgents {
				headers := make(http.Header)
				headers.Add("X-Scope-OrgID", projectID)

				query := fmt.Sprintf("{file_type=\"%v\"}", name)
				logs, err := helpers.GetLogs(cli, logsAddr, query, since1h, projectID)
				Expect(err).ToNot(HaveOccurred())

				if len(logs.Data.Result) == 0 {
					logsNotFound[name] = struct{}{}
				}
			}
			Expect(logsNotFound).To(BeEmpty(), "%v logs not found", logsNotFound)
		})

		It("OpenTelemetry Collector logs should be present in edgenode loki", func() {
			query := "{file_type=\"OpenTelemetry_Collector\"}"
			logs, err := helpers.GetLogs(cli, logsAddr, query, since1h, projectID)
			Expect(err).ToNot(HaveOccurred())

			Expect(logs.Data.Result).ToNot(BeEmpty())
		})

		It("Telegraf logs should be present in edgenode loki", func() {
			query := "{file_type=\"Telegraf\"}"
			logs, err := helpers.GetLogs(cli, logsAddr, query, since1h, projectID)
			Expect(err).ToNot(HaveOccurred())

			Expect(logs.Data.Result).ToNot(BeEmpty())
		})

		It("Caddy logs should be present in edgenode loki", func() {
			query := "{file_type=\"caddy\"}"
			logs, err := helpers.GetLogs(cli, logsAddr, query, since1h, projectID)
			Expect(err).ToNot(HaveOccurred())

			Expect(logs.Data.Result).ToNot(BeEmpty())
		})

		It("Apt Install logs should be present in edgenode loki", func() {
			query := "{file_type=\"AptInstallLogs\"}"
			logs, err := helpers.GetLogs(cli, logsAddr, query, since3h, projectID)
			Expect(err).ToNot(HaveOccurred())

			Expect(logs.Data.Result).ToNot(BeEmpty())
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
