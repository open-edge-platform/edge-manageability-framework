package main

import (
	"fmt"
	"os"
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
