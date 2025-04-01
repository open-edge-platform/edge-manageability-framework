// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package autocert_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bitfield/script"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Autocert Tests", func() {
	Describe("Autocert Tests", Ordered, Label("autocert"), func() {
		cmd := script.Exec("kubectl logs -n orch-gateway pod/cert-synchronizer-set-0")
		output, err := cmd.String()

		It("should return pod logs without error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create a tls-orch secret in the orch-gateway namespace on Init", func() {
			Expect(strings.Contains(output, "[createK8sCertificateSecret] Successfully updated Kubernetes secret tls-orch in namespace")).To(BeTrue())
		})

		It("should update the AWS cert updated on Init", func() {
			Expect(strings.Contains(output, "[doInititalCertUpdate] Successfully updated certificate:")).To(BeTrue())
		})

		PIt("should renew cert without error", func() {
			cmd = script.Exec("cmctl renew  -n orch-gateway kubernetes-docker-internal")
			output, err = cmd.String()
			Expect(err).ToNot(HaveOccurred())
		})

		It("should wait for cert to update and wait for pod logs to show success.", func() {
			endTime := time.Now().Local().Add(time.Minute * time.Duration(3))
			cmd = script.Exec("kubectl logs --since=5m -n orch-gateway pod/cert-synchronizer-set-0")
			// waits for logs to show cewrt updated in the past 5 minutes. times out after 3 minutes
			for !strings.Contains(output, "[UpdateCert] Successfully updated certificate:") && time.Now().Before(endTime) {
				cmd = script.Exec("kubectl logs --since=5m -n orch-gateway pod/cert-synchronizer-set-0")
				output, err = cmd.String()
				time.Sleep(5 * time.Second)
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.Contains(output, "[UpdateCert] Successfully updated certificate:")).To(BeTrue())
		})

		It("should have both secrets tls-autocert and tls-orch ", func() {
			cmd = script.Exec("kubectl get secrets -n orch-gateway")
			output, err = cmd.String()

			Expect(err).ToNot(HaveOccurred())
			Expect(strings.Contains(output, "tls-autocert")).To(BeTrue())
			Expect(strings.Contains(output, "tls-orch")).To(BeTrue())
		})
	})
})

func TestAutocert(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Autocert Suite")
}
