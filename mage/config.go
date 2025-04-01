// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// type proxyConfig struct {
// 	HTTPProxy          string `yaml:"httpProxy,omitempty"`
// 	HTTPSProxy         string `yaml:"httpsProxy,omitempty"`
// 	NoProxy            string `yaml:"noProxy,omitempty"`
// 	EnHTTPProxy        string `yaml:"enHttpProxy,omitempty"`
// 	EnHTTPSProxy       string `yaml:"enHttpsProxy,omitempty"`
// 	EnNoProxy          string `yaml:"enNoProxy,omitempty"`
// 	NoPeerProxyDomains string `yaml:"noPeerProxyDomains,omitempty"`
// }

// type clusterSettings struct {
// 	Name                string   `yaml:"name"`
// 	ID                  string   `yaml:"id"`
// 	EnableObservability bool     `yaml:"enableObservability,omitempty" default:"true"`
// 	DeployURL           string   `yaml:"deployUrl"`
// 	ProxyConfigFile     string   `yaml:"proxyConfigFile"`
// 	JumpHostAllowList   []string `yaml:"jumpHostAllowList"`
// 	RegistryCache       string   `yaml:"registryCache"`
// 	RegistryCacheCert   string   `yaml:"registryCacheCert"`
// }

// // UnmarshalYAML is a custom unmarshaler to set default values.
// func (c *clusterSettings) UnmarshalYAML(unmarshal func(interface{}) error) error {
// 	type alias clusterSettings
// 	defaults := alias{
// 		EnableObservability: true,
// 	}
// 	if err := unmarshal(&defaults); err != nil {
// 		return err
// 	}
// 	*c = clusterSettings(defaults)
// 	return nil
// }

// loadClusterSettings loads a clusterSettings object from a provided YAML file path.
// func loadClusterSettings(yamlPath string) (*clusterSettings, error) {
// 	data, err := os.ReadFile(yamlPath)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to read YAML file: %w", err)
// 	}

// 	var settings clusterSettings
// 	if err := yaml.Unmarshal(data, &settings); err != nil {
// 		return nil, fmt.Errorf("failed to unmarshal YAML into clusterSettings: %w", err)
// 	}

// 	if settings.ProxyConfigFile != "" {
// 		proxyConfigPath := fmt.Sprintf("%s/%s", os.DirFS(yamlPath), settings.ProxyConfigFile)
// 		proxyData, err := os.ReadFile(proxyConfigPath)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to read proxy config file: %w", err)
// 		}

// 		var proxy proxyConfig
// 		if err := yaml.Unmarshal(proxyData, &proxy); err != nil {
// 			return nil, fmt.Errorf("failed to unmarshal proxy config: %w", err)
// 		}

// 		settings.Proxy = &proxy
// 	}

// 	fmt.Printf("Loaded cluster settings: %+v\n", settings)

// 	return &settings, nil
// }

// Create a cluster deployment configuration.
func getClusterSettings() (map[string]interface{}, error) {
	clusterValues := make(map[string]interface{})

	if _, err := fmt.Println("Create a cluster deployment configuration"); err != nil {
		return clusterValues, nil
	}

	clusterValues["name"] = "default-cluster"
	clusterValues["id"] = "dev"
	clusterValues["enableObservability"] = true
	clusterValues["enableKyverno"] = true
	clusterValues["enableEdgeInfra"] = true
	clusterValues["enableAutoProvision"] = true

	return clusterValues, nil
}

func (c Config) createCluster() (string, error) {
	clusterSettings, err := getClusterSettings()
	if err != nil {
		return "", fmt.Errorf("invalid cluster settings: %w", err)
	}

	// TBD: render the new settings to a appropriate file(s)

	// Render the cluster deployment configuration template.
	// templatePath := "orch-configs/template/cluster.tpl"
	// tmpl, err := template.ParseFiles(templatePath)
	// if err != nil {
	// 	return fmt.Errorf("failed to parse template: %w", err)
	// }

	// outputPath := "orch-configs/clusters/cluster.yaml"
	// outputFile, err := os.Create(outputPath)
	// if err != nil {
	// 	return fmt.Errorf("failed to create output file: %w", err)
	// }
	// defer outputFile.Close()

	// if err := tmpl.Execute(outputFile, clusterSettings); err != nil {
	// 	return fmt.Errorf("failed to render template: %w", err)
	// }

	name, ok := clusterSettings["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("invalid cluster settings: missing or invalid 'name'")
	}

	return name, nil
}

// writeMapAsYAML writes a map[string]interface{} as a YAML string.
func writeMapAsYAML(data map[string]interface{}) (string, error) {
	var sb strings.Builder
	encoder := yaml.NewEncoder(&sb)
	encoder.SetIndent(2)

	if err := encoder.Encode(data); err != nil {
		return "", fmt.Errorf("failed to encode map as YAML: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("failed to close YAML encoder: %w", err)
	}

	return sb.String(), nil
}

// deepMerge performs a deep merge of newValuesMap into baseMap.
func deepMerge(baseMap, newValuesMap map[string]interface{}) {
	for key, newValue := range newValuesMap {
		if baseValue, exists := baseMap[key]; exists {
			// If both values are maps, perform a recursive merge.
			baseMapAsMap, baseIsMap := baseValue.(map[string]interface{})
			newValueAsMap, newIsMap := newValue.(map[string]interface{})
			if baseIsMap && newIsMap {
				deepMerge(baseMapAsMap, newValueAsMap)
			} else {
				// Overwrite the value in baseMap if it's not a map or types differ.
				baseMap[key] = newValue
			}
		} else {
			// Add the new value to baseMap if it doesn't exist.
			baseMap[key] = newValue
		}
	}
}

// parseClusterValues loads and merges values from a cluster configuration file and its referenced files.
func parseClusterValues(clusterConfigPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(clusterConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster configuration file: %w", err)
	}

	var rootConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &rootConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster configuration: %w", err)
	}

	clusterValues := make(map[string]interface{})
	if root, ok := rootConfig["root"].(map[string]interface{}); ok {
		if clusterValuesPaths, ok := root["clusterValues"].([]interface{}); ok {
			for _, path := range clusterValuesPaths {
				filePath, ok := path.(string)
				if !ok {
					return nil, fmt.Errorf("invalid clusterValues entry, expected string but got %T", path)
				}
				// fmt.Printf("Loading cluster values file: %s\n", filePath)
				fileData, err := os.ReadFile(filePath)
				if err != nil {
					return nil, fmt.Errorf("failed to read cluster values file '%s': %w", filePath, err)
				}

				var fileValues map[string]interface{}
				if err := yaml.Unmarshal(fileData, &fileValues); err != nil {
					return nil, fmt.Errorf("failed to unmarshal cluster values from file '%s': %w", filePath, err)
				}
				if filePath != clusterConfigPath {
					// fmt.Printf("Loading and merging cluster values file: %s\n", filePath)
					// Perform a deep merge of fileValues into clusterValues.
					deepMerge(clusterValues, fileValues)
				} else {
					// Remove 'clusterValues' from the root config prior to merging into the consolidated clusterValues.
					if root, ok := fileValues["root"].(map[string]interface{}); ok {
						delete(root, "clusterValues")
					}
					deepMerge(clusterValues, fileValues)
				}
			}
		} else {
			return nil, fmt.Errorf("invalid cluster definition: 'clusterValues' list is missing in the configuration")
		}
	} else {
		return nil, fmt.Errorf("invalid cluster definition: 'root' key is missing in the configuration")
	}

	return clusterValues, nil
}

// Create a cluster deployment configuration from a cluster values file.
func (Config) usePreset(clusterPresetFile string) (string, error) {
	clusterTemplatePath := "orch-configs/templates/cluster.tpl"
	clusterTmpl, err := template.ParseFiles(clusterTemplatePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	clusterValues, err := os.ReadFile(clusterPresetFile)
	if err != nil {
		return "", fmt.Errorf("failed to read cluster preset file: %w", err)
	}

	// fmt.Printf("Cluster values for debugging:\n%s\n", string(clusterValues))

	var presetData map[string]interface{}
	if err := yaml.Unmarshal(clusterValues, &presetData); err != nil {
		return "", fmt.Errorf("failed to unmarshal yaml: %w", err)
	}

	// Add default values if not specified in the parsed presetData.
	defaults := map[string]interface{}{
		"targetCluster":       "kind",
		"enableObservability": true,
		"enableKyverno":       true,
		"enableEdgeInfra":     true,
		"enableAutoProvision": true,
		"proxyProfile":        "",
		"deployProfile":       "dev",
		"enableTraefikLogs":   true,
		"enableMailpit":       false,
		"dockerCache":         "",
		"dockerCacheCert":     "",
	}

	// Merge default values into the presetData map.
	for key, defaultValue := range defaults {
		if _, exists := presetData[key]; !exists {
			presetData[key] = defaultValue
		}
	}

	if _, ok := presetData["name"]; !ok {
		return "", fmt.Errorf("missing required field 'name' in cluster values")
	}
	clusterName := presetData["name"].(string)

	presetDataValues := map[string]interface{}{
		"Values": presetData,
	}

	outputPath := fmt.Sprintf("orch-configs/clusters/%s.yaml", clusterName)
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	} else {
		fmt.Printf("Cluster values file created: %s\n", outputPath)
	}
	defer outputFile.Close()

	if err := clusterTmpl.Execute(outputFile, presetDataValues); err != nil {
		return "", fmt.Errorf("failed to render cluster template: %w", err)
	}

	if proxyProfile, ok := presetData["proxyProfile"].(string); ok && proxyProfile != "" {
		proxyProfilePath := fmt.Sprintf("%s/%s", filepath.Dir(clusterPresetFile), proxyProfile)
		proxyValuesData, err := os.ReadFile(proxyProfilePath)
		if err != nil {
			return "", fmt.Errorf("failed to read proxy profile file '%s': %w", proxyProfilePath, err)
		}

		var proxyData map[string]interface{}
		if err := yaml.Unmarshal(proxyValuesData, &proxyData); err != nil {
			return "", fmt.Errorf("failed to unmarshal proxy profile: %w", err)
		}

		proxyTemplatePath := "orch-configs/templates/proxy.tpl"
		proxyTmpl, err := template.ParseFiles(proxyTemplatePath)
		if err != nil {
			return "", fmt.Errorf("failed to parse proxy template: %w", err)
		}

		proxyOutputPath := fmt.Sprintf("orch-configs/profiles/proxy-%s.yaml", clusterName)
		proxyOutputFile, err := os.Create(proxyOutputPath)
		if err != nil {
			return "", fmt.Errorf("failed to create proxy output file: %w", err)
		} else {
			fmt.Printf("Proxy profile file created: %s\n", proxyOutputPath)
		}
		defer proxyOutputFile.Close()

		proxyValues := map[string]interface{}{
			"Values": proxyData,
		}

		if err := proxyTmpl.Execute(proxyOutputFile, proxyValues); err != nil {
			return "", fmt.Errorf("failed to render proxy template: %w", err)
		}
	}

	return clusterName, nil
}

// Create a cluster values file using the cluster configuration interface.
func (Config) createPreset() error {
	if _, err := fmt.Printf("Create a cluster values file using the cluster configuration interface\n"); err != nil {
		return err
	}

	// TBD: Prompt for a file name to store the preset file

	return nil
}

func (Config) clean() error {
	gitignorePath := "orch-configs/clusters/.gitignore"
	clusterDir := "orch-configs/clusters"

	// Load .gitignore patterns.
	gitignoreData, err := os.ReadFile(gitignorePath)
	if err != nil {
		return fmt.Errorf("failed to read .gitignore file: %w", err)
	}
	gitignorePatterns := []string{}
	for _, line := range strings.Split(string(gitignoreData), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			gitignorePatterns = append(gitignorePatterns, line)
		}
	}

	// Walk through the cluster directory and process .yaml files.
	files, err := os.ReadDir(clusterDir)
	if err != nil {
		return fmt.Errorf("failed to read cluster directory: %w", err)
	}

	for _, file := range files {
		// Skip directories.
		if file.IsDir() {
			continue
		}

		// Check if the file is a .yaml file.
		if filepath.Ext(file.Name()) == ".yaml" {
			// Check if the file matches any .gitignore pattern and is not explicitly included.
			relPath := file.Name()

			matched := false
			var negated bool
			for _, pattern := range gitignorePatterns {
				if strings.HasPrefix(pattern, "!") {
					negatedPattern := pattern[1:]
					negated, err = filepath.Match(negatedPattern, relPath)
					if err != nil {
						return err
					}
					if negated {
						matched = false
						break
					}
				} else {
					m, err := filepath.Match(pattern, relPath)
					if err != nil {
						return err
					}
					if m {
						matched = true
					}
				}
			}

			if matched {
				fmt.Printf("Deleting ignored file: %s\n", filepath.Join(clusterDir, relPath))
				if err := os.Remove(filepath.Join(clusterDir, relPath)); err != nil {
					return fmt.Errorf("failed to delete file '%s': %w", relPath, err)
				}

				// Check for and delete the corresponding proxy profile file.
				proxyFileName := fmt.Sprintf("proxy-%s.yaml", strings.TrimSuffix(relPath, filepath.Ext(relPath)))
				proxyFilePath := filepath.Join("orch-configs/profiles", proxyFileName)
				if _, err := os.Stat(proxyFilePath); err == nil {
					fmt.Printf("Deleting associated proxy profile file: %s\n", proxyFilePath)
					if err := os.Remove(proxyFilePath); err != nil {
						return fmt.Errorf("failed to delete proxy profile file '%s': %w", proxyFilePath, err)
					}
				}
			}
		}
	}
	if err != nil {
		return fmt.Errorf("error while processing cluster directory: %w", err)
	}

	return nil
}

func (Config) getTargetEnvType(targetEnv string) (string, error) {
	if targetEnv == "" {
		return "", fmt.Errorf("target environment is not specified")
	}

	clusterFilePath := fmt.Sprintf("orch-configs/clusters/%s.yaml", targetEnv)
	clusterValues, err := parseClusterValues(clusterFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse cluster values: %w", err)
	}

	targetCluster, ok := clusterValues["orchestratorDeployment"].(map[string]interface{})["targetCluster"].(string)
	if !ok || targetCluster == "" {
		targetCluster = "kind"
	}
	return targetCluster, nil
}

func (Config) isAutoCertEnabled(targetEnv string) (bool, error) {
	if targetEnv == "" {
		return false, fmt.Errorf("target environment is not specified")
	}

	clusterFilePath := fmt.Sprintf("orch-configs/clusters/%s.yaml", targetEnv)
	clusterValues, err := parseClusterValues(clusterFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to parse cluster values: %w", err)
	}

	argoConfig, ok := clusterValues["argo"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("'argo' configuration is missing or invalid")
	}

	autoCertConfig, ok := argoConfig["autocert"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("'autocert' configuration is missing or invalid")
	}

	enabled, ok := autoCertConfig["enabled"].(bool)
	if !ok {
		return false, fmt.Errorf("'enabled' field is missing or not a boolean")
	}

	return enabled, nil
}

func (Config) debug(targetEnv string) error {
	if _, err := fmt.Printf("Debugging cluster configuration for target environment: %s\n", targetEnv); err != nil {
		return err
	}

	clusterFilePath := fmt.Sprintf("orch-configs/clusters/%s.yaml", targetEnv)
	clusterValues, err := parseClusterValues(clusterFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse cluster values: %w", err)
	}

	clusterValuesYaml, err := writeMapAsYAML(clusterValues)
	if err != nil {
		return fmt.Errorf("failed to write cluster values as YAML: %w", err)
	}
	fmt.Print(clusterValuesYaml)

	return nil
}
