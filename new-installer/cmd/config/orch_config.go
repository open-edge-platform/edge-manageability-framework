// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/open-edge-platform/edge-manageability-framework/installer/asset"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/spf13/cobra"
)

type flag struct {
	Debug              bool
	PackagePath        string
	ConfigPath         string
	NonInteractiveMode bool
	VerifyMode         bool
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
var (
	flags                flag
	orchPackages         map[string]config.OrchPackage
	tmpJumpHostWhitelist string
	tmpEKSIAMRoles       string
	enabledSimple        []string
	enabledAdvanced      []string
	configMode           Mode
)

func loadOrchPackages() {
	if flags.PackagePath != "" {
		// If a package path is provided, we will load from that file
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
	} else {
		// If no package path is provided, we will load from the embedded file
		bytes, err := asset.EmbedPackage.ReadFile("packages.yaml")
		if err != nil {
			fmt.Printf("Failed to read embedded packages.yaml: %v\n", err)
			os.Exit(1)
		}
		err = yaml.Unmarshal(bytes, &orchPackages)
		if err != nil {
			fmt.Printf("Failed to decode orchestrator packages string: %v\n", err)
			os.Exit(1)
		}
	}
}

func loadConfig() {
	file, err := os.Open(flags.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Config file does not exist. Starting fresh...")
			input.Version = config.UserConfigVersion
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

	preProcessConfig()
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

	if fileVersion >= config.MinUserConfigVersion && fileVersion <= config.UserConfigVersion {
		// Version is compatible to the latest. No migration needed
		if err := yaml.Unmarshal(yamlBytes, &input); err != nil {
			return fmt.Errorf("failed to decode config file into version %d: %w", fileVersion, err)
		}
		input.Version = config.UserConfigVersion
	} else {
		return fmt.Errorf("unsupported config file version: %d", fileVersion)
	}
	return nil
}

func saveConfig() {
	postProcessConfig()

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
		fmt.Printf("%+v\n\n", input)
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

func preProcessConfig() {
	// Convert slice to comma separated string
	tmpJumpHostWhitelist = config.SliceToCommaSeparated(input.AWS.JumpHostWhitelist)
	tmpEKSIAMRoles = config.SliceToCommaSeparated(input.AWS.EKSIAMRoles)
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

	// Convert comma separated field into a slice
	input.AWS.JumpHostWhitelist = config.CommaSeparatedToSlice(tmpJumpHostWhitelist)
	input.AWS.EKSIAMRoles = config.CommaSeparatedToSlice(tmpEKSIAMRoles)

	// Setting up default values
	if input.Orch.DefaultPassword == "" {
		input.Orch.DefaultPassword = "ChangeMeOn1stLogin!"
	}
}

func main() {
	var tmpOrchName string
	var tmpScale string

	cobraCmd := &cobra.Command{
		Use:   "config-builder",
		Short: "An interactive tool to build EMF config",
		Run: func(cmd *cobra.Command, args []string) {
			loadOrchPackages()
			loadConfig()

			if flags.NonInteractiveMode {
				scale, err := strconv.Atoi(tmpScale)
				if err != nil {
					fmt.Printf("Invalid scale value: %s\n", tmpScale)
					os.Exit(1)
				}
				input.Global.OrchName = tmpOrchName
				input.Global.Scale = config.Scale(scale)
			}
			if flags.NonInteractiveMode || flags.VerifyMode {
				if err := validateAll(); err != nil {
					fmt.Println("Validation failed:", err)
					fmt.Println("Please run the command without --auto or --verify to fix the issues.")
					os.Exit(1)
				}
			} else {
				if err := orchInstallerForm().Run(); err != nil {
					fmt.Println("Failed to generate config:", err)
					os.Exit(1)
				}
			}

			saveConfig()
		},
	}

	cobraCmd.PersistentFlags().BoolVarP(&flags.Debug, "debug", "d", false, "Enable debug mode")
	cobraCmd.PersistentFlags().StringVarP(&flags.ConfigPath, "config", "c", "configs.yaml", "Path to the config file")
	cobraCmd.PersistentFlags().StringVarP(&flags.PackagePath, "package", "p", "", "Path to the Orchestrator package definition")
	cobraCmd.PersistentFlags().BoolVar(&flags.NonInteractiveMode, "auto", false, "Generate config in non-interactive mode. Requires ORCH_NAME and ORCH_SCALE environment variables to be set. This should only be used for automated testing")
	cobraCmd.PersistentFlags().BoolVar(&flags.VerifyMode, "verify", false, "Verify config in non-interactive mode without generating it")
	cobraCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// Note: Think twice before adding new env var for auto mode
		// Config file should be the only source of truth when possible
		autoFlag := cmd.Flags().Changed("auto")
		if autoFlag {
			tmpOrchName, _ = os.LookupEnv("ORCH_NAME")
			tmpScale, _ = os.LookupEnv("ORCH_SCALE")
			if tmpOrchName == "" || tmpScale == "" {
				fmt.Println("Environment variables ORCH_NAME and ORCH_SCALE must be set when using --auto")
				os.Exit(1)
			}
		}
		return nil
	}

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
