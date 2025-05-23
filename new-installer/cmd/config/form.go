// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

func configureGlobal() *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("Orchestrator Name").
			Description("Name of this orchestrator deployment").
			Placeholder("demo").
			Validate(validateOrchName).
			Value(&input.Global.OrchName),
		huh.NewInput().
			Title("Parent Domain").
			Description("Parent domain name. The domain for this deployment will be orchName.parentDomain").
			Placeholder("edgeorchestration.intel.com").
			Validate(validateParentDomain).
			Value(&input.Global.ParentDomain),
		huh.NewInput().
			Title("Admin Email").
			Description("Admin email address. This will be used to sign certificate and deliver alerts").
			Placeholder("firstname.lastname@intel.com").
			Validate(validateAdminEmail).
			Value(&input.Global.AdminEmail),
		huh.NewSelect[config.Scale]().
			Title("Scale").
			Description("Select target scale").
			Options(
				huh.NewOption("1~10 Edge Nodes", config.Scale10),
				huh.NewOption("10~100 Edge Nodes", config.Scale100),
				huh.NewOption("100-500 Edge Nodes", config.Scale500),
				huh.NewOption("500-1000 Edge Nodes", config.Scale1000),
				huh.NewOption("1000-10000 Edge Nodes", config.Scale10000),
			).
			Value(&input.Global.Scale),
	).Title("Step 1: Global Settings\n")
}

func configureProvider() *huh.Group {
	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("Infrastructure Type").
			Value(&input.Provider).
			Description("Select the infrastructure type where the EMF will be deployed.").
			Options(
				huh.NewOption("AWS", "aws"),
				huh.NewOption("On-Premises", "onprem"),
			),
	).Title("Step 2: Infrastructure Type\n")
}

func configureAwsBasic() *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("AWS Region").
			Description("This is the region where the EMF will be deployed.").
			Placeholder("us-east-1").
			Validate(validateAwsRegion).
			Value(&input.AWS.Region),
	).WithHideFunc(func() bool {
		return input.Provider != "aws"
	}).Title("Step 3a: AWS Basic Configuration\n")
}

func confirmAwsExpert() *huh.Group {
	return huh.NewGroup(
		huh.NewConfirm().
			Title("Proceed with AWS Expert Configuration?").
			Description("Skip it if you are not sure").
			Affirmative("Configure").
			Negative("Skip").
			Value(&flags.ConfigureAwsExpert),
	).WithHideFunc(func() bool {
		return flags.ExpertMode || input.Provider != "aws"
	}).Title("Step 3b: (Optional) AWS Expert Configurations\n")
}

func configureAwsExpert() *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("Custom Tag").
			Description("(Optional) Apply this tag to all AWS resources").
			Placeholder("").
			Validate(validateAwsCustomTag).
			Value(&input.AWS.CustomerTag),
		huh.NewInput().
			Title("Container Registry Cache").
			Description("(Optional) Pull OCI artifact from this cache registry").
			Placeholder("").
			Validate(validateCacheRegistry).
			Value(&input.AWS.CacheRegistry),
		huh.NewInput().
			Title("Jump Host Whitelist").
			Description("(Optional) Traffic from this CIDR will be allowed to access the jump host").
			Placeholder("10.0.0.0/8").
			Validate(validateAwsJumpHostWhitelist).
			Value(&tmpJumpHostWhitelist),
		huh.NewInput().
			Title("VPC ID").
			Description("(Optional) Enter VPC ID if you prefer to reuse existing VPC instead of letting us create one").
			Placeholder("").
			Validate(validateAwsVpcId).
			Value(&input.AWS.VPCID),
		huh.NewConfirm().
			Title("Reduce NS TTL").
			Description("(Optional) Reduce the TTL of the NS record to 60 seconds").
			Affirmative("yes").
			Negative("no").
			Value(&input.AWS.ReduceNSTTL),
		huh.NewInput().
			Title("EKS DNS IP").
			Description("(Optional) Enter EKS DNS IP if you prefer to reuse a non-default DNS server").
			Placeholder("").
			Validate(validateAwsEksDnsIp).
			Value(&input.AWS.EKSDNSIP),
	).WithHideFunc(func() bool {
		return input.Provider != "aws" || (!flags.ExpertMode && !flags.ConfigureAwsExpert)
	}).Title("Step 3b: (Optional) AWS Expert Configurations\n")
}

func configureOnPremBasic() *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("Argo CD IP Address").
			Description("This is the IP address of Argo CD.").
			Placeholder("192.168.1.1").
			Validate(validateIp).
			Value(&input.Onprem.ArgoIP),
		huh.NewInput().
			Title("Traefik IP Address").
			Description("This is the IP address of Traefik.").
			Placeholder("192.168.1.2").
			Validate(validateIp).
			Value(&input.Onprem.TraefikIP),
		huh.NewInput().
			Title("NGINX IP Address").
			Description("This is the IP address of NGINX.").
			Placeholder("192.168.1.3").
			Validate(validateIp).
			Value(&input.Onprem.NginxIP),
	).WithHideFunc(func() bool {
		return input.Provider != "onprem"
	}).Title("Step 3a: Enter On-Prem Configuration\n")
}

func confirmOnPremExpert() *huh.Group {
	return huh.NewGroup(
		huh.NewConfirm().
			Title("Proceed with OnPrem Expert Configuration?").
			Description("Skip it if you are not sure").
			Affirmative("Configure").
			Negative("Skip").
			Value(&flags.ConfigureOnPremExpert),
	).WithHideFunc(func() bool {
		return flags.ExpertMode || input.Provider != "onprem"
	}).Title("Step 3b: (Optional) OnPrem Expert Configurations\n")
}

func configureOnPremExpert() *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("Docker Username").
			Description("Docker username to be used for pulling OCI artifacts").
			Placeholder("").
			Value(&input.Onprem.DockerUsername),
		huh.NewInput().
			Title("Docker Token").
			Description("Docker token to be used for pulling OCI artifacts").
			Placeholder("").
			Value(&input.Onprem.DockerToken),
	).WithHideFunc(func() bool {
		return input.Provider != "onprem" || (!flags.ExpertMode && !flags.ConfigureOnPremExpert)
	}).Title("Step 3b: (Optional) On-Prem Expert Configurations\n")
}

func confirmProxy() *huh.Group {
	return huh.NewGroup(
		huh.NewConfirm().
			Title("Proceed with Proxy Configuration?").
			Description("This is only required when running in a network behind proxy.").
			Affirmative("Configure").
			Negative("Skip").
			Value(&flags.ConfigureProxy),
	).WithHideFunc(func() bool {
		return flags.ExpertMode
	}).Title("Step 4: (Optional) Proxy\n")
}

func configureProxy() *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("HTTP Proxy").
			Description("(Optional) HTTP proxy to be used for all outbound traffic").
			Placeholder("").
			Validate(validateProxy).
			Value(&input.Proxy.HTTPProxy),
		huh.NewInput().
			Title("HTTPS Proxy").
			Description("(Optional) HTTPS proxy to be used for all outbound traffic").
			Placeholder("").
			Validate(validateProxy).
			Value(&input.Proxy.HTTPSProxy),
		huh.NewInput().
			Title("SOCKS Proxy").
			Description("(Optional) SOCKS proxy to be used for all outbound traffic").
			Placeholder("").
			Validate(validateProxy).
			Value(&input.Proxy.SocksProxy),
		huh.NewInput().
			Title("No Proxy").
			Description("(Optional) Comma separated list of domains that should not use the proxy").
			Placeholder("").
			Validate(validateProxy).
			Value(&input.Proxy.NoProxy),
	).WithHideFunc(func() bool {
		return !flags.ExpertMode && !flags.ConfigureProxy
	}).Title("Step 4: (Optional) Proxy\n")
}

func confirmCert() *huh.Group {
	return huh.NewGroup(
		huh.NewConfirm().
			Title("Proceed with TLS Certificate Configuration?").
			Description("You can provide TLS certificate, or we will generate one using LetsEncrypt").
			Affirmative("Configure").
			Negative("Skip").
			Value(&flags.ConfigureCert),
	).WithHideFunc(func() bool {
		return flags.ExpertMode
	}).Title("Step 5: (Optional) TLS Certificate\n")
}

func configureCert() *huh.Group {
	return huh.NewGroup(
		huh.NewText().
			Title("TLS Certificate").
			Description("(Optional) TLS certificate to be used for the EMF").
			Placeholder("").
			Validate(validateTlsCert).
			Value(&input.Cert.TLSCert),
		huh.NewText().
			Title("TLS Key").
			Description("(Optional) TLS key to be used for the EMF").
			Placeholder("").
			Validate(validateTlsKey).
			Value(&input.Cert.TLSKey),
		huh.NewText().
			Title("TLS CA").
			Description("(Optional) TLS CA to be used for the EMF").
			Placeholder("").
			Validate(validateTlsCa).
			Value(&input.Cert.TLSCA),
	).WithHideFunc(func() bool {
		return !flags.ExpertMode && !flags.ConfigureCert
	}).Title("Step 5: (Optional) TLS Certificate\n")
}

func confirmSre() *huh.Group {
	return huh.NewGroup(
		huh.NewConfirm().
			Title("Proceed with SRE Configuration?").
			Description("Skip it if you are not sure").
			Affirmative("Configure").
			Negative("Skip").
			Value(&flags.ConfigureSre),
	).WithHideFunc(func() bool {
		return flags.ExpertMode
	}).Title("Step 6: (Optional) Site Reliability Engineering (SRE)\n")
}

func configureSre() *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("SRE Username").
			Description("(Optional) SRE username to be used for the EMF").
			Placeholder("").
			Value(&input.SRE.Username),
		huh.NewInput().
			Title("SRE Password").
			Description("(Optional) SRE password to be used for the EMF").
			Placeholder("").
			EchoMode(huh.EchoModePassword).
			Value(&input.SRE.Password),
		huh.NewInput().
			Title("SRE Secret URL").
			Description("(Optional) SRE secret URL to be used for the EMF").
			Placeholder("").
			Validate(validateSreSecretUrl).
			Value(&input.SRE.SecretUrl),
		huh.NewInput().
			Title("SRE CA Secret").
			Description("(Optional) SRE CA secret to be used for the EMF").
			Placeholder("").
			Validate(validateSreCaSecret).
			Value(&input.SRE.CASecret),
	).WithHideFunc(func() bool {
		return !flags.ExpertMode && !flags.ConfigureCert
	}).Title("Step 6: (Optional) Site Reliability Engineering (SRE)\n")
}

func confirmSmtp() *huh.Group {
	return huh.NewGroup(
		huh.NewConfirm().
			Title("Proceed with Email notification configuration?").
			Description("Skip it if you are not sure").
			Affirmative("Configure").
			Negative("Skip").
			Value(&flags.ConfigureSmtp),
	).WithHideFunc(func() bool {
		return flags.ExpertMode
	}).Title("Step 7: (Optional) Email Notification\n")
}

func configureSmtp() *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("SMTP Username").
			Description("(Optional) SMTP username to be used for the EMF").
			Placeholder("").
			Value(&input.SMTP.Username),
		huh.NewInput().
			Title("SMTP Password").
			Description("(Optional) SMTP password to be used for the EMF").
			Placeholder("").
			EchoMode(huh.EchoModePassword).
			Value(&input.SMTP.Password),
		huh.NewInput().
			Title("SMTP URL").
			Description("(Optional) SMTP URL to be used for the EMF").
			Placeholder("").
			Validate(validateSmtpUrl).
			Value(&input.SMTP.URL),
		huh.NewInput().
			Title("SMTP Port").
			Description("(Optional) SMTP port to be used for the EMF").
			Placeholder("").
			Validate(validateSmtpPort).
			Value(&input.SMTP.Port),
		huh.NewInput().
			Title("SMTP From Address").
			Description("(Optional) SMTP from address to be used for the EMF").
			Placeholder("").
			Validate(validateSmtpFrom).
			Value(&input.SMTP.From),
	).WithHideFunc(func() bool {
		return !flags.ExpertMode && !flags.ConfigureSmtp
	}).Title("Step 7: (Optional) Email Notification\n")
}

func orchConfigMode() *huh.Group {
	options := []huh.Option[Mode]{
		huh.NewOption("Simple   - select from pre-defined packages (recommended)", Simple),
		huh.NewOption("Advanced - enable/disable each individual apps", Advanced),
	}
	if len(input.Orch.Enabled) != 0 {
		options = append(options, huh.NewOption("Skip     - use existing config", Skip))
	}
	return huh.NewGroup(
		huh.NewSelect[Mode]().
			Title("Orchestrator Configuration Mode").
			Description("Warning: Simple mode will reset all the advanced settings that was previously configured").
			Options(options...).
			Value(&configMode),
	).Title("Step 8: Orchestrator Configuration\n")
}

func simpleMode() *huh.Group {
	var options []huh.Option[string]
	// Collect all apps into a slice for sorting
	packageList := make([]struct {
		Name    string
		Package config.OrchPackage
	}, 0, len(orchPackages))
	for name, pkg := range orchPackages {
		packageList = append(packageList, struct {
			Name    string
			Package config.OrchPackage
		}{name, pkg})
	}
	// Sort by package.Name alphabetically
	slices.SortFunc(packageList, func(a, b struct {
		Name    string
		Package config.OrchPackage
	}) int {
		return strings.Compare(a.Package.Name, b.Package.Name)
	})
	for _, item := range packageList {
		options = append(options,
			huh.NewOption(fmt.Sprintf("%s (%s)", item.Package.Name, item.Package.Description), item.Name).
				Selected(true),
		)
	}

	return huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select Orchestrator Packages").
			Description("Select the orchestrator packages to be enabled in the EMF.").
			Options(options...).
			Value(&enabledSimple).
			Validate(validateSimpleMode),
	).WithHideFunc(func() bool {
		return configMode != Simple
	}).Title("Step 8: Select Orchestrator Components (Simple Mode)\n")
}

func advancedMode() *huh.Group {
	var options []huh.Option[string]
	// Collect all apps from all packages
	appList := make([]struct {
		Name string
		App  config.OrchApp
	}, 0)
	for _, pkg := range orchPackages {
		for name, app := range pkg.Apps {
			appList = append(appList, struct {
				Name string
				App  config.OrchApp
			}{name, app})
		}
	}
	// Sort by app.Name alphabetically
	slices.SortFunc(appList, func(a, b struct {
		Name string
		App  config.OrchApp
	}) int {
		return strings.Compare(a.App.Name, b.App.Name)
	})
	for _, item := range appList {
		options = append(options,
			huh.NewOption(fmt.Sprintf("%s (%s)", item.App.Name, item.App.Description), item.Name).
				Selected(slices.Contains(input.Orch.Enabled, item.Name)),
		)
	}

	return huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select Orchestrator Components").
			Description("Select the Orchestrator components to be enabled in the EMF.").
			Options(options...).
			Value(&enabledAdvanced).
			Validate(validateAdvancedMode).
			Height(25),
	).WithHideFunc(func() bool {
		return configMode != Advanced
	}).Title("Step 8: Select Orchestrator Components (Advanced Mode)\n")
}

func orchInstallerForm() *huh.Form {
	return huh.NewForm(
		configureGlobal(),
		configureProvider(),
		configureAwsBasic(),
		confirmAwsExpert(),
		configureAwsExpert(),
		configureOnPremBasic(),
		confirmOnPremExpert(),
		configureOnPremExpert(),
		confirmProxy(),
		configureProxy(),
		confirmCert(),
		configureCert(),
		confirmSre(),
		configureSre(),
		confirmSmtp(),
		configureSmtp(),
		orchConfigMode(),
		simpleMode(),
		advancedMode(),
	).WithTheme(huh.ThemeCharm())
}
