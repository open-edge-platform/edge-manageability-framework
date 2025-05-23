package main

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"gopkg.in/yaml.v3"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/spf13/cobra"
)

const version = 2

type flag struct {
	Debug       bool
	PackagePath string
	ConfigPath  string
	ExpertMode  bool
	// Flags to show optional configurations
	ConfigureAwsExpert    bool
	ConfigureOnPremExpert bool
	ConfigureProxy        bool
	ConfigureCert         bool
	ConfigureSre          bool
	ConfigureSmtp         bool
}

type Mode int

const (
	Simple Mode = iota
	Advanced
	Skip
)

// These are states that will be saved back to the config file
var input config.OrchInstallerConfig

// These are intermediate states that will not be saved back to the config file
var flags flag
var orchPackages map[string]config.OrchPackage
var tmpJumpHostWhitelist string
var enabledSimple []string
var enabledAdvanced []string
var configMode Mode

func loadOrchPackages() {
	file, err := os.Open(flags.PackagePath)
	if err != nil {
		fmt.Printf("Failed to open orchestrator packages file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&orchPackages)
	if err != nil {
		fmt.Printf("Failed to decode orchestrator packages file: %v\n", err)
		os.Exit(1)
	}
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
	).Title("Step 1: Infrastructure Type\n")
}

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
			OptionsFunc(
				func() []huh.Option[config.Scale] {
					var options []huh.Option[config.Scale]
					options = append(options,
						huh.NewOption("1~10 Edge Nodes", config.Scale10),
						huh.NewOption("10~100 Edge Nodes", config.Scale100),
						huh.NewOption("100-500 Edge Nodes", config.Scale500),
						huh.NewOption("500-1000 Edge Nodes", config.Scale1000),
						huh.NewOption("1000-10000 Edge Nodes", config.Scale10000),
					)
					return options
				}, nil).
			Value(&input.Global.Scale),
	).Title("Step 2: Global Settings\n")
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
	}).Title("Step 3b: AWS Expert Configurations\n")
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
	}).Title("Step 5: (Optional) Site Reliability Engineering (SRE)\n")
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
	}).Title("Step 6: (Optional) Email Notification\n")
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
	}).Title("Step 6: (Optional) Email Notification\n")
}

func orchConfigMode() *huh.Group {
	return huh.NewGroup(
		huh.NewSelect[Mode]().
			Title("Orchestrator Configuration Mode").
			Description("Warning: Simple mode will reset all the advanced settings that was previously configured").
			OptionsFunc(func() []huh.Option[Mode] {
				var options []huh.Option[Mode]
				options = append(options,
					huh.NewOption("Simple   - select from pre-defined packages (recommended)", Simple),
					huh.NewOption("Advanced - enable/disable each individual apps", Advanced),
				)
				if len(input.Orch.Enabled) != 0 {
					options = append(options,
						huh.NewOption("Skip     - use existing config", Skip),
					)
				}
				return options
			}, nil).
			Value(&configMode),
	).Title("Step 7: Orchestrator Configuration\n")
}

func simpleMode() *huh.Group {
	return huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select Orchestrator Packages").
			Description("Select the orchestrator packages to be enabled in the EMF.").
			OptionsFunc(func() []huh.Option[string] {
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
				return options
			}, nil).
			Value(&enabledSimple).
			Validate(validateSimpleMode),
	).WithHideFunc(func() bool {
		return configMode != Simple
	}).Title("Step 7: Select Orchestrator Components (Simple Mode)\n")
}

func advancedMode() *huh.Group {
	return huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select Orchestrator Components").
			Description("Select the Orchestrator components to be enabled in the EMF.").
			OptionsFunc(
				func() []huh.Option[string] {
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
					return options
				},
				nil,
			).
			Value(&enabledAdvanced).
			Validate(validateAdvancedMode).
			Height(25),
	).WithHideFunc(func() bool {
		return configMode != Advanced
	}).Title("Step 7: Select Orchestrator Components (Advanced Mode)\n")
}

func loadConfig() {
	file, err := os.Open(flags.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Config file does not exist. Starting fresh...")
			input.Version = version
			return
		}
		fmt.Println("Failed to open config file:", err)
		os.Exit(1)
	}
	defer file.Close()

	// Don't know the version yet. Read in as generic map[string]interface{}
	var raw map[string]interface{}
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&raw)
	if err != nil {
		fmt.Println("Failed to decode config file for migration:", err)
		os.Exit(1)
	}

	// Migrate existing config
	if err = migrateConfig(raw); err != nil {
		fmt.Println("Failed to migrate existing config:", err)
		os.Exit(1)
	}
}

func migrateConfig(raw map[string]interface{}) error {
	var v interface{}
	var ok bool
	var fileVersion int

	yamlBytes, _ := yaml.Marshal(raw)

	if v, ok = raw["version"]; !ok {
		return fmt.Errorf("version not found in config file")
	}
	if fileVersion, ok = v.(int); !ok {
		return fmt.Errorf("version is not an integer in config file")
	}

	if fileVersion == version {
		// Version is the latest. No migration needed
		if err := yaml.Unmarshal(yamlBytes, &input); err != nil {
			return fmt.Errorf("failed to decode config file into version %d: %s", fileVersion, err)
		}
	} else {
		return fmt.Errorf("unsupported config file version: %d", fileVersion)
	}
	return nil
}

func saveConfig() {
	file, err := os.Create(flags.ConfigPath)
	if err != nil {
		fmt.Println("Failed to create config file:", err)
		os.Exit(1)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	defer encoder.Close()

	err = encoder.Encode(&input)
	if err != nil {
		fmt.Println("Failed to encode config file:", err)
		os.Exit(1)
	}

	if flags.Debug {
		fmt.Printf("%+v\n", input)
		encoder = yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		defer encoder.Close()

		err = encoder.Encode(&input)
		if err != nil {
			fmt.Println("Failed to encode config to stdout:", err)
			os.Exit(1)
		}
	}
}

func postProcessConfig() {
	// Convert input.Orch.Enabled when using simple mode
	if configMode == Simple {
		enabledAdvanced = []string{}
		for _, pkg := range enabledSimple {
			for appName := range orchPackages[pkg].Apps {
				enabledAdvanced = append(enabledAdvanced, appName)
			}
		}
		input.Orch.Enabled = enabledAdvanced
	}
	if configMode == Advanced {
		input.Orch.Enabled = enabledAdvanced
	}

	// Covert comma separated IPs into a slice
	if tmpJumpHostWhitelist != "" {
		parts := strings.Split(tmpJumpHostWhitelist, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		input.AWS.JumpHostWhitelist = parts
	}

	// Setting up default values
	if input.Orch.DefaultPassword == "" {
		input.Orch.DefaultPassword = "ChangeMeOn1stLogin!"
	}
}

func validateOrchName(s string) error {
	if len(s) == 0 {
		return fmt.Errorf("orchestrator name cannot be empty")
	}
	if len(s) >= 16 {
		return fmt.Errorf("orchestrator name must be less than 16 characters")
	}
	if matched := regexp.MustCompile(`^[a-z0-9]+$`).MatchString(s); !matched {
		return fmt.Errorf("orchestrator name must be all lower case letters or digits")
	}
	return nil
}

func validateParentDomain(s string) error {
	if matched := regexp.MustCompile(`^[a-z0-9.-]+$`).MatchString(s); !matched {
		return fmt.Errorf("parent domain must be all lower case letters, digits, or '.'")
	}
	return nil
}

func validateAdminEmail(s string) error {
	if matched := regexp.MustCompile(`^[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}$`).MatchString(s); !matched {
		return fmt.Errorf("admin email must be a valid email address")
	}
	return nil
}

func validateAwsRegion(s string) error {
	if matched := regexp.MustCompile(`^[a-z]+-[a-z]+-\d$`).MatchString(s); !matched {
		return fmt.Errorf("region must follow the format '^[a-z]+-[a-z]+-\\d$', e.g., 'us-west-2'")
	}
	return nil
}

func validateAwsCustomTag(s string) error {
	return nil
}

func validateCacheRegistry(s string) error {
	return nil
}

func validateAwsJumpHostWhitelist(s string) error {
	return nil
}

func validateAwsVpcId(s string) error {
	if s == "" {
		return nil
	}
	if matched := regexp.MustCompile(`^vpc-[0-9a-f]{8}$`).MatchString(s); !matched {
		return fmt.Errorf("VPC ID must follow the format '^vpc-[0-9a-f]{8}$', e.g., 'vpc-12345678'")
	}
	return nil
}

func validateAwsEksDnsIp(s string) error {
	if s == "" {
		return nil
	}
	if matched := regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}$`).MatchString(s); !matched {
		return fmt.Errorf("EKS DNS IP must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., '")
	}
	return nil
}

func validateProxy(s string) error {
	// TODO: implement proxy validation
	return nil
}

func validateTlsCert(s string) error {
	if s == "" {
		return nil
	}
	if matched := regexp.MustCompile(`^-----BEGIN CERTIFICATE-----\n.*\n-----END CERTIFICATE-----$`).MatchString(s); !matched {
		return fmt.Errorf("TLS certificate must be in PEM format")
	}
	return nil
}

func validateTlsKey(s string) error {
	if s == "" {
		return nil
	}
	if matched := regexp.MustCompile(`^-----BEGIN PRIVATE KEY-----\n.*\n-----END PRIVATE KEY-----$`).MatchString(s); !matched {
		return fmt.Errorf("TLS key must be in PEM format")
	}
	return nil
}

func validateTlsCa(s string) error {
	if s == "" {
		return nil
	}
	if matched := regexp.MustCompile(`^-----BEGIN CERTIFICATE-----\n.*\n-----END CERTIFICATE-----$`).MatchString(s); !matched {
		return fmt.Errorf("TLS CA must be in PEM format")
	}
	return nil
}

func validateSreSecretUrl(s string) error {
	return nil
}

func validateSreCaSecret(s string) error {
	return nil
}

func validateSmtpUrl(s string) error {
	return nil
}

func validateSmtpPort(s string) error {
	if s == "" {
		return nil
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("cannot convert %s to integer: %s", s, err)
	}
	if i < 1 || i > 65535 {
		return fmt.Errorf("SMTP port must be between 1 and 65535")
	}
	return nil
}

func validateSmtpFrom(s string) error {
	return nil
}

func validateIp(s string) error {
	if matched := regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}$`).MatchString(s); !matched {
		return fmt.Errorf("IP address must follow the format '^([0-9]{1,3}\\.){3}[0-9]{1,3}$', e.g., 192.168.1.1'")
	}
	parts := strings.Split(s, ".")
	for _, part := range parts {
		i, err := strconv.Atoi(part)
		if err != nil {
			return fmt.Errorf("cannot convert %s to integer: %s", part, err)
		}
		if i < 0 || i > 255 {
			return fmt.Errorf("IP address must be between 0 and 255")
		}
	}
	return nil
}

func validateSimpleMode(s []string) error {
	if !slices.Contains(s, "fps") {
		return fmt.Errorf("FPS must be enabled")
	}
	if slices.Contains(s, "ui") &&
		!slices.Contains(s, "eim") &&
		!slices.Contains(s, "co") &&
		!slices.Contains(s, "ao") {
		return fmt.Errorf("UI cannot be enabled without at least one of EIM, AO, or CO being enabled")
	}
	return nil
}

func validateAdvancedMode(s []string) error {
	// TODO: placeholder for advanced mode validation
	return nil
}

func main() {
	var cobraCmd = &cobra.Command{
		Use:   "arctic-huh",
		Short: "An interactive tool to build EMF config",
		Run: func(cmd *cobra.Command, args []string) {
			loadOrchPackages()
			loadConfig()

			err := huh.NewForm(
				configureProvider(),
				configureGlobal(),
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
			).WithTheme(huh.ThemeCharm()).
				Run()
			if err != nil {
				fmt.Println("Failed to generate config:", err)
				os.Exit(1)
			}

			postProcessConfig()
			saveConfig()
		},
	}

	cobraCmd.PersistentFlags().BoolVarP(&flags.Debug, "debug", "d", false, "Enable debug mode")
	cobraCmd.PersistentFlags().StringVarP(&flags.ConfigPath, "config", "c", "configs.yaml", "Path to the config file")
	cobraCmd.PersistentFlags().StringVarP(&flags.PackagePath, "package", "p", "packages.yaml", "Path to the Orchestrator package definition")
	cobraCmd.PersistentFlags().BoolVarP(&flags.ExpertMode, "expert", "e", false, "Show all optional configurations")

	// Exit on help command
	helpFunc := cobraCmd.HelpFunc()
	cobraCmd.SetHelpFunc(func(cobraCmd *cobra.Command, s []string) {
		helpFunc(cobraCmd, s)
		os.Exit(1)
	})

	if err := cobraCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}
