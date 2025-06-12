// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/stretchr/testify/suite"
)

var pretty = lipgloss.NewStyle().
	Width(100).
	Border(lipgloss.NormalBorder()).
	MarginTop(1).
	Padding(1, 3, 1, 2)

var (
	form  *huh.Form
	model tea.Model
)

// Dummy SSH key file for testing
var tmpPrivKey *os.File

func (s *OrchConfigFormTest) testConfigureGlobal() {
	// Enter orchestrator name
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("demo")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter parent domain
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("edgeorchestrator.intel.com")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// enter admin email
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("admin@example.com")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Select 10~100 ENs
	model.Update(tea.KeyMsg{Type: tea.KeyDown})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 2: Infrastructure Type"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testConfigureProvider() {
	// Select AWS
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 3a: AWS Basic Configuration"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testConfigureAwsBasic() {
	// Enter AWS region
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("us-west-2")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 3b: (Optional) AWS Expert Configurations"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testConfirmAwsExpert() {
	// Select AWS expert mode
	model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 3b: (Optional) AWS Expert Configurations"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testSkipAwsExpert() {
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 4: (Optional) Proxy"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testConfigureAwsExpert() {
	// Enter custom tag
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("custom-tag")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter registry cache
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("registry-rs.edgeorchestrator.intel.com")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter just host whitelist
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("10.0.0.0/8,192.168.0.0/16")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter just host IP
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("10.20.30.1")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter just host SSH private key path
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tmpPrivKey.Name())})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter VPC ID
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("vpc-12345678")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Select Reduce NS TTL
	model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter EKS DNS IP
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("8.8.8.8")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter EKS IAM role
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("developer_eks_role")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 4: (Optional) Proxy"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testSkipProxy() {
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 5: (Optional) TLS Certificate"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testConfirmProxy() {
	model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 4: (Optional) Proxy"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testConfigureProxy() {
	// Enter EMF http proxy
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("http://proxy.example.com:8080")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter EMF https proxy
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("http://proxy.example.com:8081")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter EMF SOCKS proxy
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("http://proxy.example.com:1080")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter EMF no proxy
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(".intel.com, 10.0.0.0/8")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter EN http proxy
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("http://proxy.example.com:8080")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter EN https proxy
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("http://proxy.example.com:8081")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter EN ftp proxy
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("http://proxy.example.com:8082")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter EN SOCKS proxy
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("http://proxy.example.com:1080")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter EN no proxy
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(".intel.com ,10.0.0.0/8")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 5: (Optional) TLS Certificate"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testSkipCert() {
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 6: (Optional) Site Reliability Engineering (SRE)"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testConfirmCert() {
	model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	view := ansi.Strip(model.View())
	expected := "Step 5: (Optional) TLS Certificate"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testConfigureCert() {
	// Generate a CA certificate and key
	caPriv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTemplate := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"Test CA"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, _ := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caPriv.PublicKey, caPriv)
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	// caKeyBytes, _ := x509.MarshalECPrivateKey(caPriv)
	// caKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: caKeyBytes})

	// Generate a server certificate and key signed by the CA
	srvPriv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	srvTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	srvDER, _ := x509.CreateCertificate(rand.Reader, &srvTemplate, &caTemplate, &srvPriv.PublicKey, caPriv)
	srvPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srvDER})
	srvKeyBytes, _ := x509.MarshalECPrivateKey(srvPriv)
	srvKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: srvKeyBytes})

	// Enter TLS cert
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(string(srvPEM))})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter TLS key
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(string(srvKeyPEM))})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter TLS CA
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(string(caPEM))})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 6: (Optional) Site Reliability Engineering (SRE)"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testSkipSRE() {
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	view := ansi.Strip(model.View())
	expected := "Step 7: (Optional) Email Notification"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testConfirmSRE() {
	model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	view := ansi.Strip(model.View())
	expected := "Step 6: (Optional) Site Reliability Engineering (SRE)"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testConfigureSre() {
	// Enter SRE username
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("sre-user")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter SRE password
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("sre-password")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter SRE secret URL
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("https://sre.example.com")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter SRE CA secret
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("sre-ca-secret")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 7: (Optional) Email Notification"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testSkipSMTP() {
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 8: Orchestrator Configuration"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testConfirmSMTP() {
	model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 7: (Optional) Email Notification"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testConfigureSMTP() {
	// Enter SMTP username
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("smtp-user")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter SMTP password
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("smtp-password")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter SMTP URL
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("smtp.example.com")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter SMTP port
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("587")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter SMTP from
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("test@example.com")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 8: Orchestrator Configuration"
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testConfirmSimpleMode() {
	input.Orch.Enabled = []string{"dummy"}
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 8: Select Orchestrator Components (Simple Mode) "
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testConfirmAdvancedMode() {
	input.Orch.Enabled = []string{"dummy"}
	// Select Advanced Mode
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyDown}))
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 8: Select Orchestrator Components (Advanced Mode) "
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testSimpleMode() {
	// Select Simple Mode
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	// This is the last step so we are expecting an empty view
	view := ansi.Strip(model.View())
	expected := ""
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func (s *OrchConfigFormTest) testAdvancedMode() {
	// Select Simple Mode
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	// This is the last step so we are expecting an empty view
	view := ansi.Strip(model.View())
	expected := ""
	if !strings.Contains(view, expected) {
		s.FailNow("Expected view to contain step 2 message", "Got: %s", pretty.Render(view))
	}
}

func initTest() error {
	// Reset config builder state to avoid interference between tests
	input = config.OrchInstallerConfig{}
	flags.ConfigureAwsExpert = false
	flags.ConfigureOnPremExpert = false
	flags.ConfigureProxy = false
	flags.ConfigureCert = false
	flags.ConfigureSre = false
	flags.ConfigureSmtp = false
	configMode = Simple

	loadOrchPackages()
	form = orchInstallerForm()
	model, _ = form.Update(form.Init())

	return nil
}

type OrchConfigFormTest struct {
	suite.Suite
}

func TestConfigFormSuite(t *testing.T) {
	suite.Run(t, new(OrchConfigFormTest))
}

func (s *OrchConfigFormTest) TestSimpleWorkflow() {
	if err := initTest(); err != nil {
		s.T().Fatalf("initTest failed: %v", err)
	}

	s.Run("Configure Global", s.testConfigureGlobal)
	s.Run("Configure Provider", s.testConfigureProvider)
	s.Run("Configure AWS Basic", s.testConfigureAwsBasic)
	s.Run("Skip AWS Expert", s.testSkipAwsExpert)
	s.Run("Skip Proxy", s.testSkipProxy)
	s.Run("Skip Certificate", s.testSkipCert)
	s.Run("Skip SRE", s.testSkipSRE)
	s.Run("Skip SMTP", s.testSkipSMTP)
	s.Run("Confirm Simple Mode", s.testConfirmSimpleMode)
	s.Run("Simple Mode", s.testSimpleMode)
}

func (s *OrchConfigFormTest) TestAdvancedWorkflow() {
	if err := initTest(); err != nil {
		s.T().Fatalf("initTest failed: %v", err)
	}

	// Create a temporary file for the private key
	var err error
	tmpPrivKey, err = os.CreateTemp("/tmp", "privkey")
	if err != nil {
		s.T().Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpPrivKey.Name())

	s.Run("Configure Global", s.testConfigureGlobal)
	s.Run("Configure Provider", s.testConfigureProvider)
	s.Run("Configure AWS Basic", s.testConfigureAwsBasic)
	s.Run("Confirm AWS Expert", s.testConfirmAwsExpert)
	s.Run("Configure AWS Expert", s.testConfigureAwsExpert)
	s.Run("Confirm Proxy", s.testConfirmProxy)
	s.Run("Configure Proxy", s.testConfigureProxy)
	s.Run("Confirm Certificate", s.testConfirmCert)
	s.Run("Configure Certificate", s.testConfigureCert)
	s.Run("Confirm SRE", s.testConfirmSRE)
	s.Run("Configure SRE", s.testConfigureSre)
	s.Run("Confirm SMTP", s.testConfirmSMTP)
	s.Run("Configure SMTP", s.testConfigureSMTP)
	s.Run("Confirm Advanced Mode", s.testConfirmAdvancedMode)
	s.Run("Advanced Mode", s.testAdvancedMode)
}

// batchUpdate is a helper function to run the model and update it with the command
// Some keystroke such as Enter will trigger additional commands that
func batchUpdate(m tea.Model, cmd tea.Cmd) tea.Model {
	if cmd == nil {
		return m
	}
	msg := cmd()
	m, cmd = m.Update(msg)
	if cmd == nil {
		return m
	}
	msg = cmd()
	m, _ = m.Update(msg)
	return m
}
