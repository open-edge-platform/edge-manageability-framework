package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

var pretty = lipgloss.NewStyle().
	Width(100).
	Border(lipgloss.NormalBorder()).
	MarginTop(1).
	Padding(1, 3, 1, 2)

var form *huh.Form
var model tea.Model

func testConfigureGlobal(t *testing.T) {
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
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testConfigureProvider(t *testing.T) {
	// Select AWS
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 3a: AWS Basic Configuration"
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testConfigureAwsBasic(t *testing.T) {
	// Enter AWS region
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("us-west-2")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 3b: (Optional) AWS Expert Configurations"
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testConfirmAwsExpert(t *testing.T) {
	// Select AWS expert mode
	model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 3b: (Optional) AWS Expert Configurations"
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testSkipAwsExpert(t *testing.T) {
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 4: (Optional) Proxy"
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testConfigureAwsExpert(t *testing.T) {
	// Enter custom tag
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("custom-tag")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter registry cache
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("registry-rs.edgeorchestrator.intel.com")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter just host whitelist
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("10.0.0.0/8,192.168.0.0/16")})
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

	view := ansi.Strip(model.View())
	expected := "Step 4: (Optional) Proxy"
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testSkipProxy(t *testing.T) {
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 5: (Optional) TLS Certificate"
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testConfirmProxy(t *testing.T) {
	model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 4: (Optional) Proxy"
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testConfigureProxy(t *testing.T) {
	// Enter http proxy
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("http://proxy.example.com:8080")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter https proxy
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("http://proxy.example.com:8081")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter ftp proxy
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("http://proxy.example.com:8082")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	// Enter no proxy
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("*.intel.com")})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 5: (Optional) TLS Certificate"
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testSkipCert(t *testing.T) {
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 6: (Optional) Site Reliability Engineering (SRE)"
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testConfirmCert(t *testing.T) {
	model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	view := ansi.Strip(model.View())
	expected := "Step 5: (Optional) TLS Certificate"
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testConfigureCert(t *testing.T) {
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
	//caKeyBytes, _ := x509.MarshalECPrivateKey(caPriv)
	//caKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: caKeyBytes})

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
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testSkipSRE(t *testing.T) {
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	view := ansi.Strip(model.View())
	expected := "Step 7: (Optional) Email Notification"
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testConfirmSRE(t *testing.T) {
	model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))
	view := ansi.Strip(model.View())
	expected := "Step 6: (Optional) Site Reliability Engineering (SRE)"
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testConfigureSre(t *testing.T) {
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
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testSkipSMTP(t *testing.T) {
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 8: Orchestrator Configuration"
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testConfirmSMTP(t *testing.T) {
	model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 7: (Optional) Email Notification"
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testConfigureSMTP(t *testing.T) {
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
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testConfirmSimpleMode(t *testing.T) {
	input.Orch.Enabled = []string{"dummy"}
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 8: Select Orchestrator Components (Simple Mode) "
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testConfirmAdvancedMode(t *testing.T) {
	input.Orch.Enabled = []string{"dummy"}
	// Select Advanced Mode
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyDown}))
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	view := ansi.Strip(model.View())
	expected := "Step 8: Select Orchestrator Components (Advanced Mode) "
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testSimpleMode(t *testing.T) {
	// Select Simple Mode
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	// This is the last step so we are expecting an empty view
	view := ansi.Strip(model.View())
	expected := ""
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func testAdvancedMode(t *testing.T) {
	// Select Simple Mode
	batchUpdate(model.Update(tea.KeyMsg{Type: tea.KeyEnter}))

	// This is the last step so we are expecting an empty view
	view := ansi.Strip(model.View())
	expected := ""
	if !strings.Contains(view, expected) {
		t.Fatalf("Expected '%s'. Got: %s", expected, pretty.Render(view))
	}
}

func initTest() {
	input = config.OrchInstallerConfig{}
	flags.PackagePath = "../../assets/packages.yaml"
	loadOrchPackages()
}

func TestSimpleWorkflow(t *testing.T) {
	initTest()

	// Test the form initialization
	form = orchInstallerForm()
	model, _ = form.Update(form.Init())

	testConfigureGlobal(t)
	testConfigureProvider(t)
	testConfigureAwsBasic(t)
	testSkipAwsExpert(t)
	testSkipProxy(t)
	testSkipCert(t)
	testSkipSRE(t)
	testSkipSMTP(t)
	testSimpleMode(t)
	testAdvancedMode(t)
}

func TestAdvancedWorkflow(t *testing.T) {
	initTest()

	// Test the form initialization
	form = orchInstallerForm()
	model, _ = form.Update(form.Init())

	testConfigureGlobal(t)
	testConfigureProvider(t)
	testConfigureAwsBasic(t)
	testConfirmAwsExpert(t)
	testConfigureAwsExpert(t)
	testConfirmProxy(t)
	testConfigureProxy(t)
	testConfirmCert(t)
	testConfigureCert(t)
	testConfirmSRE(t)
	testConfigureSre(t)
	testConfirmSMTP(t)
	testConfigureSMTP(t)
	testConfirmAdvancedMode(t)
	testAdvancedMode(t)
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
