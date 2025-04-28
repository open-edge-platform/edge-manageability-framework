// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Add default values if not specified in the parsed presetData.
var (
	defaultPresetValues = map[string]interface{}{
		"targetCluster":       "kind",
		"clusterDomain":       serviceDomain,
		"argoServiceType":     "LoadBalancer",
		"enableObservability": true,
		"enableAuditLogging":  false,
		"enableKyverno":       true,
		"enableEdgeInfra":     true,
		"enableAutoProvision": true,
		"proxyProfile":        "",
		"deployProfile":       "dev",
		"enableTraefikLogs":   true,
		"enableMailpit":       false,
		"dockerCache":         "",
		"dockerCacheCert":     "",
		"deployRepoURL":       "https://gitea-http.gitea.svc.cluster.local/argocd/edge-manageability-framework",
	}
)

func (c Config) createCluster() (string, error) {
	fmt.Println("Interactive cluster configuration is not currently supported.")
	fmt.Println("Use config:usePreset with a manually generated preset file until this functionality is supported.")

	// TBD: Implement interactive queryClusterPresetSettings interface
	// clusterSettings, err := queryClusterPresetSettings()
	// if err != nil {
	// 	return "", fmt.Errorf("invalid cluster settings: %w", err)
	// }

	// Render the cluster deployment configuration template.
	// clusterName, err := renderClusterTemplate(clusterSettings)
	// return clusterName, nil

	return "", nil
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
				fileData, err := os.ReadFile(filePath)
				if err != nil {
					return nil, fmt.Errorf("failed to read cluster values file '%s': %w", filePath, err)
				}

				var fileValues map[string]interface{}
				if err := yaml.Unmarshal(fileData, &fileValues); err != nil {
					return nil, fmt.Errorf("failed to unmarshal cluster values from file '%s': %w", filePath, err)
				}
				if filePath != clusterConfigPath {
					deepMerge(clusterValues, fileValues)
				} else {
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
	clusterValues, err := os.ReadFile(clusterPresetFile)
	if err != nil {
		return "", fmt.Errorf("failed to read cluster preset file: %w", err)
	}

	var presetData map[string]interface{}
	if err := yaml.Unmarshal(clusterValues, &presetData); err != nil {
		return "", fmt.Errorf("failed to unmarshal yaml: %w", err)
	}

	// Apply defaultPresetValues to the presetData map to fill in any missing defaults.
	for key, defaultValue := range defaultPresetValues {
		if _, exists := presetData[key]; !exists {
			presetData[key] = defaultValue
		}
	}

	// Evaluate and update proxyProfile relative path if it exists.
	if proxyProfile, ok := presetData["proxyProfile"].(string); ok && proxyProfile != "" {
		proxyProfilePath := fmt.Sprintf("%s/%s", filepath.Dir(clusterPresetFile), proxyProfile)
		presetData["proxyProfile"] = proxyProfilePath
	}

	var clusterName string
	if clusterName, err = renderClusterTemplate(presetData); err != nil {
		return "", fmt.Errorf("failed to render cluster template: %w", err)
	}

	return clusterName, nil
}

// Render cluster preset data into a template and save it to a cluster definition file.
func renderClusterTemplate(presetData map[string]interface{}) (string, error) {
	// Use "name" from the presetData to handle file naming in alignment with existing targetEnv logic
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

	clusterTemplatePath := "orch-configs/templates/cluster.tpl"
	if err := renderTemplate(clusterTemplatePath, presetDataValues, outputFile); err != nil {
		return "", fmt.Errorf("failed to render cluster template: %w", err)
	}

	if proxyProfile, ok := presetData["proxyProfile"].(string); ok && proxyProfile != "" {
		proxyValuesData, err := os.ReadFile(proxyProfile)
		if err != nil {
			return "", fmt.Errorf("failed to read proxy profile file '%s': %w", proxyProfile, err)
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

func renderTemplate(templatePath string, vars any, out io.Writer) error {
	tmpl := template.New("template").Funcs(template.FuncMap{
		"indent": func(spaces int, v string) string {
			prefix := strings.Repeat(" ", spaces)
			return prefix + strings.ReplaceAll(v, "\n", "\n"+prefix)
		},
		"toYaml": func(v interface{}) (string, error) {
			out, err := yaml.Marshal(v)
			if err != nil {
				return "", err
			}
			return string(out), nil
		},
	})
	tmpl, err := tmpl.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, filepath.Base(templatePath), vars); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if _, err := out.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}

// Create a cluster values file using the cluster configuration interface.
func (Config) createPreset() error {
	if _, err := fmt.Printf("Create a cluster values file using the cluster configuration interface\n"); err != nil {
		return err
	}

	// TBD: Prompt for a file name to store the preset file

	return nil
}

// Remove ignored files based on .gitignore set from the orch-configs directory
func (Config) clean() error {
	gitignorePath := "orch-configs/.gitignore"
	configsDir := "orch-configs"

	// Load .gitignore patterns.
	gitignoreData, err := os.ReadFile(gitignorePath)
	if err != nil {
		return fmt.Errorf("failed to read .gitignore file: %w", err)
	}

	excludePatterns := []string{}
	includePatterns := []string{}
	for _, line := range strings.Split(string(gitignoreData), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "!") {
				// Negated pattern (include)
				includePatterns = append(includePatterns, strings.TrimPrefix(line, "!"))
			} else {
				// Regular pattern (exclude)
				excludePatterns = append(excludePatterns, line)
			}
		}
	}

	// Walk through the cluster directory and process .yaml files.
	err = filepath.Walk(configsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".yaml" {
			relPath, err := filepath.Rel(configsDir, path)
			if err != nil {
				return err
			}

			excludeMatch := false
			includeMatch := false
			for _, pattern := range excludePatterns {
				matched, err := filepath.Match(pattern, relPath)
				if err != nil {
					return fmt.Errorf("error matching pattern '%s': %w", pattern, err)
				}
				if matched {
					excludeMatch = true
					break
				}
			}

			for _, pattern := range includePatterns {
				matched, err := filepath.Match(pattern, relPath)
				if err != nil {
					return fmt.Errorf("error matching pattern '%s': %w", pattern, err)
				}
				if matched {
					includeMatch = true
					break
				}
			}

			if excludeMatch && !includeMatch {
				fmt.Printf("Deleting ignored file: %s\n", path)
				if err := os.Remove(path); err != nil {
					return fmt.Errorf("failed to delete file '%s': %w", path, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error while enumerating .yaml files: %w", err)
	}

	return nil
}

func (Config) getTargetValues(targetEnv string) (map[string]interface{}, error) {
	if targetEnv == "" {
		return nil, fmt.Errorf("target environment is not specified")
	}

	clusterFilePath := fmt.Sprintf("orch-configs/clusters/%s.yaml", targetEnv)
	targetValues, err := parseClusterValues(clusterFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cluster values: %w", err)
	}

	return targetValues, nil
}

func (c Config) getTargetEnvType(targetEnv string) (string, error) {
	defaultEnv := "kind"

	clusterValues, err := c.getTargetValues(targetEnv)
	if err != nil {
		return defaultEnv, fmt.Errorf("failed to get target values: %w", err)
	}

	orchestratorDeploymentConfig, ok := clusterValues["orchestratorDeployment"].(map[string]interface{})
	if !ok {
		return defaultEnv, fmt.Errorf("'orchestratorDeployment' configuration is missing or invalid")
	}

	targetCluster, ok := orchestratorDeploymentConfig["targetCluster"].(string)
	if !ok || targetCluster == "" {
		return defaultEnv, fmt.Errorf("'targetCluster' field is missing or empty")
	}

	return targetCluster, nil
}

func (c Config) isAutoCertEnabled(targetEnv string) (bool, error) {
	clusterValues, err := c.getTargetValues(targetEnv)
	if err != nil {
		return false, fmt.Errorf("failed to get target values: %w", err)
	}

	argoConfig, ok := clusterValues["argo"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("'argo' configuration is missing or invalid")
	}

	autoCertConfig, ok := argoConfig["autoCert"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("'autoCert' configuration is missing or invalid")
	}

	enabled, ok := autoCertConfig["enabled"].(bool)
	if !ok {
		return false, fmt.Errorf("'enabled' field is missing or not a boolean")
	}

	return enabled, nil
}

func (c Config) isMailpitEnabled(targetEnv string) (bool, error) {
	clusterValues, err := c.getTargetValues(targetEnv)
	if err != nil {
		return false, fmt.Errorf("failed to get target values: %w", err)
	}

	orchestratorDeploymentConfig, ok := clusterValues["orchestratorDeployment"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("'orchestratorDeployment' configuration is missing or invalid")
	}

	enableMailpit, ok := orchestratorDeploymentConfig["enableMailpit"].(bool)
	if !ok {
		return false, fmt.Errorf("'enableMailpit' field is missing or not a boolean")
	}

	return enableMailpit, nil
}

func (c Config) getDockerCache(targetEnv string) (string, error) {
	clusterValues, err := c.getTargetValues(targetEnv)
	if err != nil {
		return "", fmt.Errorf("failed to get target values: %w", err)
	}

	orchestratorDeploymentConfig, ok := clusterValues["orchestratorDeployment"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("'orchestratorDeployment' configuration is missing or invalid")
	}

	dockerCache, ok := orchestratorDeploymentConfig["dockerCache"].(string)
	if !ok {
		return "", fmt.Errorf("'dockerCache' field is missing or not a boolean")
	}

	return dockerCache, nil
}

func (c Config) getDockerCacheCert(targetEnv string) (string, error) {
	clusterValues, err := c.getTargetValues(targetEnv)
	if err != nil {
		return "", fmt.Errorf("failed to get target values: %w", err)
	}

	orchestratorDeploymentConfig, ok := clusterValues["orchestratorDeployment"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("'orchestratorDeployment' configuration is missing or invalid")
	}

	dockerCacheCert, ok := orchestratorDeploymentConfig["dockerCacheCert"].(string)
	if !ok {
		return "", fmt.Errorf("'dockerCacheCert' field is missing or not a boolean")
	}

	return dockerCacheCert, nil
}

func (c Config) renderTargetConfigTemplate(targetEnv string, templatePath string, outputPath string) error {
	targetValues, err := c.getTargetValues(targetEnv)
	if err != nil {
		return fmt.Errorf("failed to get target values: %w", err)
	}

	templateValues := map[string]interface{}{
		"Values": targetValues,
	}
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()
	if err := tmpl.Execute(outputFile, templateValues); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}
	fmt.Printf("Rendered target configuration file: %s\n", outputPath)

	return nil
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
