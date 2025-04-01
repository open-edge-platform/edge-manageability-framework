// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	// "text/template"
	"path/filepath"
	"time"
	"unicode"

	"github.com/bitfield/script"
	"gopkg.in/yaml.v3"
)

func (Gen) dockerImageManifest() error {
	data, err := script.NewPipe().Exec("docker exec -t kind-control-plane crictl images").String()
	if err != nil {
		return fmt.Errorf("getting docker image manifest: %w \n%s", err, data)
	}
	fmt.Printf("%s\nDocker manifest generated. 🐋\n", data)
	return nil
}

type ComponentDetails struct {
	AppName     string `yaml:"deployment_name"`
	Chart       string `yaml:"chart"`
	Namespace   string `yaml:"namespace"`
	Order       string `yaml:"order"`
	ReleaseName string `yaml:"release_name"`
	Repo        string `yaml:"repo"`
	Version     string `yaml:"version"`
}

type Manifest struct {
	Components        map[string]ComponentDetails `yaml:"components"`
	Date              string                      `yaml:"date"`
	EdgeNodeGitHash   string                      `yaml:"edgeNodeGitHash"`
	EdgeNodeGitOrigin string                      `yaml:"edgeNodeGitOrigin"`
	GitHash           string                      `yaml:"gitHash"`
	GitOrigin         string                      `yaml:"gitOrigin"`
	Tag               string                      `yaml:"tag"`
	Time              string                      `yaml:"time"`
	Type              string                      `yaml:"type"`
}

func getIndentSize(str string) int {
	indent := 0
	for _, char := range str {
		if !unicode.IsSpace(char) {
			break
		}
		indent++
	}
	return indent
}

func getGitOriginPath(dir string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func getGitCommitDate(repoPath string, commitHash string) (string, error) {
	cmdArgs := []string{"-C", repoPath, "log", "-1", "--format=%cI"}
	if commitHash != "" {
		cmdArgs = append(cmdArgs, commitHash)
	}

	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

func splitOnLastSlash(str string) (string, string) {
	lastSlashIndex := strings.LastIndex(str, "/")
	if lastSlashIndex == -1 {
		return "", str
	}
	return str[:lastSlashIndex], str[lastSlashIndex+1:]
}

func parseAppConfig(filename string) (*ComponentDetails, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("unable to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	values := make(map[string]string)
	var config ComponentDetails
	fmValueRegex := regexp.MustCompile(`{{-\s+\$([\w]+)\s+:=\s+"([^"]+)".*}}`)
	inFrontmatter := true
	inSpec := false
	inSources := false
	sourcesIndent := 2

	for scanner.Scan() {
		line := scanner.Text()
		if inFrontmatter {
			if line == "---" {
				inFrontmatter = false
				continue
			}
			matches := fmValueRegex.FindStringSubmatch(line)
			if len(matches) != 3 {
				continue
			}
			values[matches[1]] = matches[2]
			if matches[1] == "appName" {
				config.AppName = matches[2]
			}
			if matches[1] == "namespace" {
				config.Namespace = matches[2]
			}
			if matches[1] == "syncWave" {
				config.Order = matches[2]
			}
		} else {
			if strings.HasPrefix(line, "spec:") {
				inSpec = true
				continue
			}
			if inSpec {
				if sourcesIndent = strings.Index(line, "sources:"); sourcesIndent > 0 {
					inSpec = false
					inSources = true
					continue
				}
			}
			if inSources {
				indent := getIndentSize(line)
				if indent <= sourcesIndent && line[indent] != '-' {
					break
				}
				match := regexp.MustCompile(`repoURL:\s*(.*)$`).FindStringSubmatch(line)
				if match != nil {
					if strings.Contains(match[1], ".Values.argo.chartRepoURL") {
						config.Repo = "oci://registry-rs.edgeorchestration.intel.com/edge-orch"
					} else if strings.Contains(match[1], ".Values.argo.rsChartRepoURL") {
						config.Repo = "oci://registry-rs.edgeorchestration.intel.com/edge-orch"
					} else {
						config.Repo = strings.Trim(match[1], " ")
					}
					continue
				}
				match = regexp.MustCompile(`targetRevision:\s*(.*)$`).FindStringSubmatch(line)
				if match != nil {
					config.Version = strings.Trim(match[1], " ")
					continue
				}
				match = regexp.MustCompile(`chart:\s*(.*)$`).FindStringSubmatch(line)
				if match != nil {
					chartStr := strings.Trim(match[1], " ")
					if _, ok := values["appName"]; ok {
						chartStr = regexp.MustCompile(`{{\s*\$appName\s*}}`).ReplaceAllString(chartStr, values["appName"])
					}
					if _, ok := values["chartName"]; ok {
						chartStr = regexp.MustCompile(`{{\s*\$chartName\s*}}`).ReplaceAllString(chartStr, values["chartName"])
					}
					path, chart := splitOnLastSlash(strings.Trim(chartStr, " "))
					config.Chart = chart
					if path != "" {
						config.Repo = config.Repo + "/" + path
					}
					continue
				}
				match = regexp.MustCompile(`path:\s*(.*)$`).FindStringSubmatch(line)
				if match != nil {
					pathStr := strings.Trim(match[1], " ")
					if _, ok := values["appName"]; ok {
						pathStr = regexp.MustCompile(`{{\s*\$appName\s*}}`).ReplaceAllString(pathStr, values["appName"])
					}
					if _, ok := values["chartName"]; ok {
						pathStr = regexp.MustCompile(`{{\s*\$chartName\s*}}`).ReplaceAllString(pathStr, values["chartName"])
					}
					config.Chart = pathStr
					continue
				}
				match = regexp.MustCompile(`releaseName:\s*(.*)$`).FindStringSubmatch(line)
				if match != nil {
					if strings.Contains(match[1], "$appName") {
						config.ReleaseName = values["appName"]
					} else {
						config.ReleaseName = strings.Trim(match[1], " ")
					}
					continue
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &config, nil
}

func getManifest() (*Manifest, error) {
	var manifest Manifest
	var err error

	repoDir := getDeployDir()
	configsDir := getConfigsDir()
	if _, err := os.Stat(configsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("invalid config directory: %s", configsDir)
	}

	manifest.Components = make(map[string]ComponentDetails)
	manifest.Type = "argocd"

	manifest.GitHash = getDeployRevision()
	manifest.GitOrigin, err = getGitOriginPath(repoDir)
	if err != nil {
		return nil, fmt.Errorf("unable to get deploy repo origins: %w", err)
	}

	commitDateTime, err := getGitCommitDate(repoDir, manifest.GitHash)
	if err != nil {
		return nil, fmt.Errorf("unable to get deploy commit time: %w", err)
	}
	t, err := time.Parse(time.RFC3339, commitDateTime)
	if err != nil {
		return nil, fmt.Errorf("unable to parse deploy commit time: %w", err)
	}
	utc := t.UTC()
	manifest.Date = utc.Format("2006-01-02")
	manifest.Time = utc.Format("15:04:05")
	manifest.Tag = utc.Format("20060102.15")

	appPath := filepath.Join(repoDir, "argocd", "applications", "templates")
	appEntries, err := os.ReadDir(appPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read app configs: %w", err)
	}

	for _, appfile := range appEntries {
		if !appfile.IsDir() && strings.HasSuffix(appfile.Name(), ".yaml") {
			config, err := parseAppConfig(filepath.Join(appPath, appfile.Name()))
			if err != nil {
				fmt.Println("  error parsing app config: %w", err)
			} else {
				if config.ReleaseName == "" {
					config.ReleaseName = config.AppName
				}
				manifest.Components[config.AppName] = *config
			}
		}
	}

	return &manifest, nil
}

// save a release manifest file that contains all included helm component versions.
func (Gen) releaseManifest(manifestFilename string) error {
	manifest, err := getManifest()
	if err != nil {
		return fmt.Errorf("error creating manifest: %w", err)
	}

	manifestFile, err := os.Create(manifestFilename)
	if err != nil {
		return fmt.Errorf("unable to create manifest file: %w", err)
	}
	defer manifestFile.Close()

	yamlManifest, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("error encoding manifest: %w", err)
	}

	_, err = manifestFile.Write(yamlManifest)
	if err != nil {
		return fmt.Errorf("error writing manifest: %w", err)
	}

	fmt.Println("Created release manifest:", manifestFilename)
	return nil
}

// print out a release manifest that contains all included helm component versions.
func (Gen) dumpReleaseManifest() error {
	manifest, err := getManifest()
	if err != nil {
		return fmt.Errorf("error creating manifest: %w", err)
	}

	yamlManifest, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("error encoding manifest: %w", err)
	}

	fmt.Printf("%s\n", yamlManifest)
	return nil
}

// pull a specified helm file version artifact to a local directory.
func helmPullImage(imagePath string, version string, targetDir string) error {
	// DBG: fmt.Println("helm pull", imagePath, "--version", version)
	cmd := exec.Command("helm", "pull", imagePath, "--version", version)
	cmd.Dir = targetDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error pulling image: %w", err)
	}
	fmt.Println(string(output))

	return nil
}

// mergeValuesMap recursively merges two Values maps together with the right overriding the left.
func mergeValuesMap(lhs, rhs map[string]interface{}) map[string]interface{} {
	mergedValues := make(map[string]interface{})

	for key, lhsValue := range lhs {
		if rhsValue, exists := rhs[key]; exists {
			// Check if both values are also maps; if so, merge recursively
			leftMap, leftOk := lhsValue.(map[string]interface{})
			rightMap, rightOk := rhsValue.(map[string]interface{})
			if leftOk && rightOk {
				lhsValue = mergeValuesMap(leftMap, rightMap)
			} else {
				lhsValue = rhsValue // Prefer right value
			}
		}
		mergedValues[key] = lhsValue
	}

	// Next, add all entries from the right map that aren't in the left map
	for key, rhsValue := range rhs {
		if _, exists := lhs[key]; !exists {
			mergedValues[key] = rhsValue
		}
	}

	return mergedValues
}

// loadValuesFile loads values from a YAML file into a map.
func loadValuesFile(filePath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var values map[string]interface{}
	err = yaml.Unmarshal(data, &values)
	if err != nil {
		return nil, err
	}

	return values, nil
}

// saveValuesFile values from a map[string]interface{} to a YAML file
func saveValuesFile(filePath string, valueMap map[string]interface{}) error {
	valuesFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating values file: %w", err)
	}
	defer valuesFile.Close()

	encoder := yaml.NewEncoder(valuesFile)
	if err := encoder.Encode(valueMap); err != nil {
		return fmt.Errorf("error writing values file: %w", err)
	}
	encoder.Close()

	return nil
}

// loadClusterConfig loads a cluster configuration that references configuration profiles.
func loadClusterConfig(cluster string) (map[string]interface{}, error) {
	clusterConfigPath := filepath.Join(getConfigsDir(), "clusters", cluster+".yaml")
	clusterConfigData, err := os.ReadFile(clusterConfigPath)
	if err != nil {
		return nil, fmt.Errorf("invalid cluster %s: %w", cluster, err)
	}

	var clusterConfig map[string]interface{}
	if err := yaml.Unmarshal(clusterConfigData, &clusterConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from file %s: %w", clusterConfigPath, err)
	}

	rootValues, ok := clusterConfig["root"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid cluster config, no root configuration")
	}

	clusterValues, ok := rootValues["clusterValues"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid cluster config, no root.clusterValues list")
	}

	returnValues := make(map[string]interface{})
	for _, profileFileElt := range clusterValues {
		profileFile, ok := profileFileElt.(string)
		if !ok {
			// return nil, fmt.Errorf("invalid profile entry:", profileFileElt)
			fmt.Println("invalid profile entry:", profileFileElt)
			continue
		}

		// profileConfigPath := filepath.Join(getConfigsDir(), profileFile)
		fmt.Println("load:", profileFile)
		profileValues, err := loadValuesFile(profileFile)
		if err != nil {
			// return nil, fmt.Errorf("error loading profile:", profileConfigPath)
			fmt.Println("error loading profile:", profileFile)
			continue
		}
		returnValues = mergeValuesMap(returnValues, profileValues)
	}

	return returnValues, nil // Return the map with no errors
}

func loadArgoAppValues(tempDir string) (map[string]interface{}, error) {
	fmt.Println("helm template argocd/applications -f", tempDir+"/values.yaml")
	cmd := exec.Command("helm", "template", "argocd/applications", "-f", tempDir+"/values.yaml")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error running helm template: %w", err)
	}

	// parse the output to extract per application valuesObject
	templates := strings.Split(string(output), "---\n")
	appValues := make(map[string]interface{})
	for _, template := range templates {
		lines := strings.Split(template, "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "# Source: argocd-applications/templates/") {
			appname := strings.TrimPrefix(lines[0], "# Source: argocd-applications/templates/")
			appname = strings.TrimSuffix(appname, ".yaml")
			yamlData := strings.Join(lines[1:], "\n")
			var values map[string]interface{}
			err := yaml.Unmarshal([]byte(yamlData), &values)
			if err != nil {
				fmt.Printf("error parsing YAML for app %s\n", appname)
				continue
			}
			valuesObjectFound := false
			if spec, ok := values["spec"].(map[string]interface{}); ok {
				if sources, ok := spec["sources"].([]interface{}); ok && len(sources) > 0 {
					if source, ok := sources[0].(map[string]interface{}); ok {
						if helm, ok := source["helm"].(map[string]interface{}); ok {
							if valuesObject, ok := helm["valuesObject"].(map[string]interface{}); ok {
								valuesObjectFound = true
								fmt.Printf("added Argo valuesObject for app %s:\n", appname)
								appValues[appname] = valuesObject
							}
						}
					}
				}
			}
			if !valuesObjectFound {
				fmt.Printf("no Argo valuesObject found for app %s\n", appname)
			}
		}
	}
	return appValues, nil
}

func helmTemplate(appName string, releaseName string, chartPath string, argoValues map[string]interface{}, outputDir string) error {
	var valueFiles []string
	err := os.MkdirAll(outputDir, 0o755)
	if err != nil {
		return fmt.Errorf("error creating output folder: %w", err)
	}

	if argoAppValues, ok := argoValues[appName].(map[string]interface{}); ok {
		appValuesFile := filepath.Join(outputDir, appName+"-values.yaml")
		valueFiles = append(valueFiles, appValuesFile)
		err := saveValuesFile(appValuesFile, argoAppValues)
		if err != nil {
			return fmt.Errorf("error saving app values: %w", err)
		}
	} else {
		baseValuesFile := filepath.Join(filepath.Dir(outputDir), "values.yaml")
		valueFiles = append(valueFiles, baseValuesFile)

		appValuesFile := "./" + filepath.Join("argocd", "applications", "configs", appName+".yaml")
		if _, err := os.Stat(appValuesFile); err != nil {
			if os.IsNotExist(err) {
				fmt.Println("app values file doesn't exist:", appValuesFile)
			} else {
				return fmt.Errorf("failed to access app values file %s: %w", appValuesFile, err)
			}
		} else {
			valueFiles = append(valueFiles, appValuesFile)
		}
	}

	cmdArgs := []string{"helm", "template", releaseName, chartPath}
	outputArgs := []string{"--output-dir", outputDir, "--debug"}
	for _, valueFile := range valueFiles {
		cmdArgs = append(cmdArgs, "-f", valueFile)
	}
	cmdArgs = append(cmdArgs, outputArgs...)

	cmdLine := strings.Join(cmdArgs, " ")
	fmt.Println(cmdLine)
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("templating error: %s, %w", output, err)
	}
	fmt.Println(string(output))

	return nil
}

const (
	hrEqual         = "======================================================================"
	hrDash          = "----------------------------------------------------------------------"
	installBasePath = "registry-rs.edgeorchestration.intel.com/edge-orch/common"
	binaryBasePath  = "registry-rs.edgeorchestration.intel.com/edge-orch/common/files"
	chartBasePath   = "registry-rs.edgeorchestration.intel.com/edge-orch/common/charts"

	// debBasePath 	= "registry-rs.edgeorchestration.intel.com/edge-orch/common/files"

	// OpenEdgePlatformContainerRegistry = OpenEdgePlatformRegistryRepoURL + "/" + OpenEdgePlatformRepository + "/" + RegistryRepoSubProj
	// OpenEdgePlatformChartRegistry     = OpenEdgePlatformRegistryRepoURL + "/" + OpenEdgePlatformRepository + "/" + RegistryRepoSubProj + "/charts" //nolint: lll
	// OpenEdgePlatformFilesRegistry     = OpenEdgePlatformRegistryRepoURL + "/" + OpenEdgePlatformRepository + "/" + RegistryRepoSubProj + "/files"  //nolint: lll
)

func parseChartFileForImageValues(filePath string) ([]string, error) {
	var values []string
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 16*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "image:") {
			// DBG: fmt.Println(line)
			re := regexp.MustCompile(`image:\s*"?([^"\s]+)"?`)
			match := re.FindStringSubmatch(line)
			if len(match) > 1 {
				value := strings.TrimSpace(match[1])
				values = append(values, value)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return values, nil
}

func uniqueSortedValues(values []string) []string {
	unique := make(map[string]bool)
	for _, value := range values {
		unique[value] = true
	}

	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}

	sort.Strings(result)
	return result
}

func parseTemplatedChartsForImageValues(rootDir string) ([]string, error) {
	var imageValues []string
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.HasSuffix(info.Name(), ".yaml") ||
			strings.HasSuffix(info.Name(), ".yml")) {
			values, err := parseChartFileForImageValues(path)
			if err != nil {
				return err
			}
			imageValues = append(imageValues, values...)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	imageValues = uniqueSortedValues(imageValues)
	return imageValues, nil
}

func filterNoProxy(noProxyEnv string, substringsToRemove []string) string {
	entries := strings.Split(noProxyEnv, ",")
	filteredEntries := []string{}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		remove := false
		for _, substring := range substringsToRemove {
			if entry == substring {
				remove = true
				break
			}
		}
		if !remove {
			filteredEntries = append(filteredEntries, entry)
		}
	}
	return strings.Join(filteredEntries, ",")
}

func removeIntelFromNoProxy() {
	noProxyEnv := os.Getenv("no_proxy")
	if noProxyEnv != "" {
		noProxyEnv = filterNoProxy(noProxyEnv, []string{"intel.com", ".intel.com"})
		os.Setenv("no_proxy", noProxyEnv)
	}
	noProxyEnv = os.Getenv("NO_PROXY")
	if noProxyEnv != "" {
		noProxyEnv = filterNoProxy(noProxyEnv, []string{"intel.com", ".intel.com"})
		os.Setenv("NO_PROXY", noProxyEnv)
	}
}

func getImageManifest() ([]string, []string, error) {
	removeIntelFromNoProxy()

	manifest, err := getManifest()
	if err != nil {
		return nil, nil, fmt.Errorf("error creating manifest: %w", err)
	}

	tempDir, err := os.MkdirTemp(".", "_appimg_*.tmp")
	if err != nil {
		return nil, nil, fmt.Errorf("error creating temp folder: %w", err)
	}
	fmt.Println("Extracting helmfiles to: ", tempDir)
	defer os.RemoveAll(tempDir)

	clusterValues, err := loadClusterConfig("bkc")
	if err != nil {
		return nil, nil, fmt.Errorf("error loading cluster configuration: %w", err)
	}

	// save clusterValues to tempDir
	valuesFilePath := filepath.Join(tempDir, "values.yaml")
	err = saveValuesFile(valuesFilePath, clusterValues)
	if err != nil {
		return nil, nil, fmt.Errorf("error saving cluster values: %w", err)
	}

	argoValues, err := loadArgoAppValues(tempDir)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading argo valueObjects: %w", err)
	}

	for _, component := range manifest.Components {
		fmt.Println(hrEqual)
		fmt.Println(component.AppName)
		fmt.Println(hrEqual)
		// DBG: fmt.Println("Repo:", component.Repo)
		if strings.HasPrefix(component.Repo, "oci://registry-rs.edgeorchestration.intel.com/edge-orch") {
			chartRemotePath := strings.Join([]string{component.Repo, component.Chart}, "/")
			err := helmPullImage(chartRemotePath, component.Version, tempDir)
			if err != nil {
				fmt.Println("error pulling helm chart for", component.AppName, ": %w", err)
			}
			chartLocalPath := filepath.Join(tempDir, filepath.Base(component.Chart)+"-"+component.Version+".tgz")
			err = helmTemplate(component.AppName, component.ReleaseName, "./"+chartLocalPath,
				argoValues, filepath.Join(tempDir, component.ReleaseName))
			if err != nil {
				fmt.Println("error templating helm chart for", component.AppName, ": %w", err)
			}
		} else {
			fmt.Println("Skipping 3rd party hosted chart")
			continue
		}
	}

	imageList, err := parseTemplatedChartsForImageValues(tempDir)
	if err != nil {
		return nil, nil, fmt.Errorf("error parsing templated charts: %w", err)
	}

	binaryList := []string{}
	// Add OCI Tarball deployment artifacts
	tarballRepos := []string{"orchestrator"}
	tarballVariants := []string{"cloudFull", "onpremFull"}
	installVariants := []string{"cloudFull"}

	deployTag, err := getDeployTag()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get tag for deployment artifacts: %w", err)
	}

	for _, variant := range tarballVariants {
		for _, repo := range tarballRepos {
			// buildImageBasePath = ${REGISTRY}"/"${REGISTRY_PROJECT}"/"${SUB_COMPONENT_NAME}"/"${ARTIFACT_TYPE}"
			// oras push $buildImageBasePath"/"${repo}"/"${VARIANT_LC}":"${TAG}"
			binaryList = append(binaryList, fmt.Sprintf("%s/%s/%s:%s", binaryBasePath, repo,
				strings.ToLower(variant), deployTag))
		}
	}

	binaryList = append(binaryList, fmt.Sprintf("%s/cloud-orchestrator-installer:%s", binaryBasePath, deployTag))

	// Add OCI installer deployment artifacts
	for _, installVariant := range installVariants {
		// buildImageBasePath = ${REGISTRY}"/"${REGISTRY_PROJECT}"/
		// oras push $buildImageBasePath"/"${repo}"/"${VARIANT_LC}":"${TAG}"
		imageList = append(imageList, fmt.Sprintf("%s/orchestrator-installer-%s:%s", installBasePath,
			strings.ToLower(installVariant), deployTag))
	}

	return imageList, binaryList, nil
}

func GetBranchName() (string, error) {
	branchName := os.Getenv("BRANCH_NAME")
	if branchName == "" {
		output, err := script.Exec("git rev-parse --abbrev-ref HEAD").String()
		if err != nil {
			return "", fmt.Errorf("get branch name: %w", err)
		}
		branchName = strings.TrimSpace(output)
	}

	// If branch name contains slashes, replace them with hyphens
	branchName = strings.ReplaceAll(branchName, "/", "-")

	return branchName, nil
}

func GetRepoVersion() (string, error) {
	contents, err := os.ReadFile("VERSION")
	if err != nil {
		return "", fmt.Errorf("read version from 'VERSION' file: %w", err)
	}
	return strings.TrimSpace(string(contents)), nil
}

func GetDebVersion() (string, error) {
	repoVersion, err := GetRepoVersion()
	if err != nil {
		return "", fmt.Errorf("read version from 'VERSION' file: %w", err)
	}

	// get branch name
	branchName, err := GetBranchName()
	if err != nil {
		return "", fmt.Errorf("get branch name: %w", err)
	}

	// check if release branch
	isReleaseBranch := branchName == "main" ||
		strings.Contains(branchName, "pass-validation") ||
		strings.HasPrefix(branchName, "release")

	// If release version on release branches, return the version as is
	if isReleaseBranch && !strings.Contains(repoVersion, "-dev") {
		return repoVersion, nil // e.g., 3.0.0, 3.0.0-rc1, 3.0.0-n20250306
	}

	stdout, err := exec.Command("git", "rev-parse", "--short", "HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get current git hash: %w", err)
	}
	shortHash := strings.TrimSpace(string(stdout))

	// else, return the version with short hash appended
	return fmt.Sprintf("%s-%s", repoVersion, shortHash), nil // e.g., 3.0.0-dev-7d763f9, 3.0.0-rc1-7d763f9 (in pre-merge CI)
}

func getOnPremDEBs() ([]string, error) {
	debList := []string{}

	matches, err := filepath.Glob(filepath.Join("on-prem-installers/dist", "*.deb"))
	if err != nil {
		return debList, fmt.Errorf("failed to list .deb files: %w", err)
	}

	// Strip dist from file paths since oras push requires the file name only or the artifact will include the entire
	// dist directory.
	var files []string
	for _, match := range matches {
		files = append(files, filepath.Base(match))
	}

	if len(files) == 0 {
		return debList, fmt.Errorf("no .deb files found in dist directory")
	}

	version, err := GetDebVersion()
	if err != nil {
		return debList, fmt.Errorf("failed to get DEB version: %w", err)
	}

	fmt.Printf("Version: %s\n", version)

	for _, file := range files {
		fmt.Printf("Processing file: %s\n", file)

		cmd := exec.Command(
			"dpkg-deb", "--showformat=${Package}", "--show", file,
		)
		cmd.Dir = "on-prem-installers/dist"

		stdouterr, err := cmd.CombinedOutput()
		if err != nil {
			return debList, fmt.Errorf("failed to get name for %s: %w: %s", file, err, string(stdouterr))
		}

		name := string(stdouterr)
		if name == "" {
			return debList, fmt.Errorf("failed to get name for %s: empty name", file)
		}

		debList = append(debList, fmt.Sprintf("%s/%s:%s", binaryBasePath, name, version))
	}

	return debList, nil
}

func getOnPremFile() (string, error) {
	version, err := GetDebVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}

	fmt.Println("Version: ", version)

	var (
		tag          = version
		artifactName = fmt.Sprintf("%s/on-prem:%s", binaryBasePath, tag)
	)

	return artifactName, nil
}

func buildOnPrem() error {
	cmd := exec.Command("mage", "build:all")
	cmd.Dir = "on-prem-installers"

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run mage build:all: %w: %s", err, string(output))
	}

	fmt.Println(string(output))
	return nil
}

func (Gen) releaseImageManifest(manifestFilename string) error {
	imageManifest, binaryManifest, err := getImageManifest()
	if err != nil {
		return fmt.Errorf("error getting image manifest: %w", err)
	}

	manifestFile, err := os.Create(manifestFilename)
	if err != nil {
		return fmt.Errorf("unable to create manifest file: %w", err)
	}
	defer manifestFile.Close()

	_, err = manifestFile.WriteString("\n")
	if err != nil {
		return fmt.Errorf("file write error: %w", err)
	}

	if len(imageManifest) == 0 {
		_, err = manifestFile.WriteString("images: []\n")
		if err != nil {
			return fmt.Errorf("file write error: %w", err)
		}
	} else {
		_, err = manifestFile.WriteString("images:\n")
		if err != nil {
			return fmt.Errorf("file write error: %w", err)
		}
		for _, imagePath := range imageManifest {
			_, err = manifestFile.WriteString("  - " + imagePath + "\n")
			if err != nil {
				return fmt.Errorf("file write error: %w", err)
			}
		}
	}

	if len(binaryManifest) == 0 {
		_, err = manifestFile.WriteString("binaries: []\n")
		if err != nil {
			return fmt.Errorf("file write error: %w", err)
		}
	} else {
		_, err = manifestFile.WriteString("binaries:\n")
		if err != nil {
			return fmt.Errorf("file write error: %w", err)
		}
		for _, binaryPath := range binaryManifest {
			_, err = manifestFile.WriteString("  - " + binaryPath + "\n")
			if err != nil {
				return fmt.Errorf("file write error: %w", err)
			}
		}
	}

	err = buildOnPrem()
	if err != nil {
		return fmt.Errorf("failed to build on-prem: %w", err)
	}

	// on-prem files
	_, err = manifestFile.WriteString("files:\n")
	if err != nil {
		return fmt.Errorf("file write error: %w", err)
	}
	onPremFile, err := getOnPremFile()
	if err != nil {
		return fmt.Errorf("failed to get on-prem file path: %w", err)
	}
	_, err = manifestFile.WriteString("  - " + onPremFile + "\n")
	if err != nil {
		return fmt.Errorf("file write error: %w", err)
	}

	// debs (on-prem)
	debs, err := getOnPremDEBs()
	if err != nil {
		return fmt.Errorf("failed to get on-prem debs: %w", err)
	}
	_, err = manifestFile.WriteString("debs:\n")
	if err != nil {
		return fmt.Errorf("file write error: %w", err)
	}
	for _, deb := range debs {
		_, err = manifestFile.WriteString("  - " + deb + "\n")
		if err != nil {
			return fmt.Errorf("file write error: %w", err)
		}
	}

	return nil
}

func (Gen) dumpReleaseImageManifest() error {
	imageManifest, binaryManifest, err := getImageManifest()
	if err != nil {
		return fmt.Errorf("error getting image manifest: %w", err)
	}
	if len(imageManifest) == 0 {
		fmt.Println("images: []")
	} else {
		fmt.Println("images:")
		for _, imagePath := range imageManifest {
			fmt.Printf("  - \"%s\"\n", imagePath)
		}
	}
	fmt.Println("")

	if len(binaryManifest) == 0 {
		fmt.Println("binaries: []")
	} else {
		fmt.Println("binaries:")
		for _, binaryPath := range binaryManifest {
			fmt.Printf("  - \"%s\"\n", binaryPath)
		}
	}

	fmt.Println("")
	return nil
}

func firewallDataLookup(dest string) (string, string, bool) {
	type lookup struct {
		Source      string
		Description string
		Skip        bool
	}
	const (
		unknownSource      = "Unknown Source"
		unknownDescription = "Unknown Description"
		northboundSource   = "Orchestrator UI/API"
		southboundSource   = "Edge Node"
	)

	data := map[string]lookup{
		"app-orch.":               {northboundSource, "Application Orchestration", false},
		"fleet.":                  {unknownSource, unknownDescription, true},
		"app-service-proxy.":      {northboundSource, "Application Orchestration", false},
		"cluster-orch-edge-node.": {unknownSource, unknownDescription, true}, // TODO: remove from cert
		"cluster-orch-node.":      {southboundSource, "Cluster Orchestration", false},
		"cluster-orch.":           {northboundSource, "Cluster Orchestration", false},
		"gitea.":                  {northboundSource, "Application Orchestration", false},
		"infra-node.":             {southboundSource, "Infrastructure Management", false},
		"attest-node.":            {southboundSource, "Infrastructure Management", false},
		"keycloak.":               {northboundSource, "Identity and Access Management", false},
		"":                        {northboundSource, "Web UI", false},
		"logs-node.":              {southboundSource, "Observability", false},
		"metadata.":               {northboundSource, "Web UI", false},
		"metrics-node.":           {southboundSource, "Observability", false},
		"observability-admin.":    {northboundSource, "Observability", false},
		"observability-ui.":       {northboundSource, "Observability", false},
		"onboarding-node.":        {southboundSource, "Infrastructure Management", false},
		"onboarding-stream.":      {southboundSource, "Infrastructure Management", false},
		"registry.":               {northboundSource, "Harbor UI", false},
		"release.":                {southboundSource, "Release Service Token", false},
		"telemetry-node.":         {southboundSource, "Observability", false},
		"tinkerbell-server.":      {southboundSource, "Onboarding", false},
		"update-node.":            {southboundSource, "Infrastructure Management", false},
		"vault.":                  {northboundSource, "Vault UI", false},
		"vnc.":                    {northboundSource, "Application Orchestration", false},
		"web-ui.":                 {northboundSource, "Web UI", false},
		"api.":                    {northboundSource, "Multi-Tenancy APIs", false},
		"ws-app-service-proxy.":   {northboundSource, "Application Orchestration", false},
		"tinkerbell-nginx.":       {southboundSource, "BIOS Onboarding", false},
		"argo.":                   {"Orchestrator Admin", "ArgoCD UI", false},
	}

	v, ok := data[dest]
	if !ok {
		return unknownSource, unknownDescription, false
	}
	return v.Source, v.Description, v.Skip
}

func findHost(hosts []string, target string) bool {
	for _, host := range hosts {
		if host == target {
			return true
		}
	}
	return false
}

func loopHosts(sb *strings.Builder, hosts []string) error {
	const (
		protocolPort  = "TCP:443"
		defaultDomain = "{domain}"
	)
	// Track hosts that were already printed
	uniqueHosts := map[string]struct{}{}

	host_order := []string{
		"",
		"web-ui",
		"api",
		"metadata",
		"app-orch",
		"app-service-proxy",
		"ws-app-service-proxy",
		"gitea",
		"vnc",
		"cluster-orch",
		"keycloak",
		"observability-admin",
		"observability-ui",
		"fleet",
		"registry",
		"vault",
		"cluster-orch-node",
		"cluster-orch-edge-node",
		"infra-node",
		"attest-node",
		"onboarding-node",
		"onboarding-stream",
		"release",
		"metrics-node",
		"telemetry-node",
		"logs-node",
		"tinkerbell-server",
		"tinkerbell-nginx",
		"update-node",
		"argo",
	}

	// Print host if unique
	for _, host := range host_order {
		h := fmt.Sprintf("%s.%s", host, serviceDomain)
		if found := findHost(hosts, h); !found {
			continue
		}
		if _, ok := uniqueHosts[h]; !ok {
			before, found := strings.CutSuffix(h, serviceDomain)
			if !found {
				return fmt.Errorf("did not find domain %s in host name %s", serviceDomain, h)
			}
			source, description, skip := firewallDataLookup(before)
			if skip {
				continue
			}
			sb.WriteString("| " + source + " | " + before + defaultDomain + " | " + protocolPort + " | " + description + " |\n")
			uniqueHosts[host] = struct{}{}
		}
	}
	return nil
}

func (Gen) firewallDoc() error {
	var sb strings.Builder
	sb.WriteString("# Firewall Configuration \n")
	sb.WriteString("\n")
	sb.WriteString("Orchestrator has the following ingress points using a separate IP address for each: \n")
	sb.WriteString("- ArgoCD Admin UI at argo.{domain}.  It is recommended that incoming traffic is restricted to a subset of known source IPs. \n")
	sb.WriteString("- BIOS Onboarding at tinkerbell-nginx.{domain}. \n")
	sb.WriteString("- All other traffic from Edge Nodes as well as UI & API users of the Orchestrator. \n")
	sb.WriteString("\n")
	sb.WriteString("| Source              | Destination | Protocol:Port | Description |\n")
	sb.WriteString("| :---------------- | :------: | :----: | :---- |\n")

	hosts, err := (Gen{}).kubeDnslookupDockerInternal()
	if err != nil {
		return err
	}
	if err := loopHosts(&sb, hosts); err != nil {
		return err
	}

	hostsBios, err := (Gen{}).kubeBiosDnslookupDockerInternal()
	if err != nil {
		return err
	}
	hostsBios = append(hostsBios, fmt.Sprintf("argo.%s", serviceDomain)) // consolidate this endpoint here

	if err := loopHosts(&sb, hostsBios); err != nil {
		return err
	}

	if _, err := script.Echo(sb.String()).WriteFile("firewall-doc.md"); err != nil {
		return err
	}

	return nil
}

func (Gen) orchestratorDomain() error {
	domain, err := LookupOrchestratorDomain()
	if err != nil {
		return err
	}

	fmt.Println(domain)

	return nil
}

func (Gen) hostfileTraefik() error {
	ip, err := lookupOrchIP()
	if err != nil {
		return err
	}
	err = (Gen{}).hostfile(ip, true)
	if err != nil {
		return err
	}

	// Add BIOS nginx hosts
	bootsIP, err := lookupGenericIP("orch-boots", "ingress-nginx-controller")
	if err != nil {
		return err
	}
	err = (Gen{}).BiosTraefikhostfile(bootsIP, true)
	if err != nil {
		return err
	}

	// Add Gitea hosts
	giteaIP, err := lookupGenericIP("gitea", "gitea-http")
	if err != nil {
		return err
	}
	err = (Gen{}).GenericHostfile(giteaIP, "gitea", true)
	if err != nil {
		return err
	}

	// Add Argo CD hosts
	argoIP, err := lookupGenericIP("argocd", "argocd-server")
	if err != nil {
		return err
	}
	err = (Gen{}).GenericHostfile(argoIP, "argo", true)
	if err != nil {
		return err
	}

	return nil
}

func (Gen) GethostSNICollection() (string, error) {
	hosts, err := (Gen{}).kubeDnslookupDockerInternal()
	if err != nil {
		return "", err
	}
	srehosts, err := Gen{}.kubeDnslookupDockerInternalSRE()
	if err != nil {
		return "", err
	}

	// Track hosts that were already printed
	uniqueHosts := map[string]struct{}{}
	hostSNICollection := "\"HostSNI("
	for _, host := range hosts {
		if _, ok := uniqueHosts[host]; !ok {
			hostSNICollection += "`" + host + "`" + ","
			uniqueHosts[host] = struct{}{}
		}
	}

	hostSNICollection += "`" + srehosts[0] + "`"
	hostSNICollection += ")\""
	return hostSNICollection, err
}

func (Gen) kubeDnslookupDockerInternal() ([]string, error) {
	// returns a sorted string of hosts
	kubeCmd := fmt.Sprintf("kubectl --v=%d -n orch-gateway get configmap kubernetes-docker-internal -o json", verboseLevel)
	hosts, err := Gen{}.dnsNamesConfigMap(kubeCmd)
	if err != nil {
		return nil, err
	}
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no ingress entries found, is orchestrator deployed?")
	}

	// Sort to make it easier for humans to search
	sort.Strings(hosts)
	return hosts, err
}

func (Gen) kubeBiosDnslookupDockerInternal() ([]string, error) {
	// returns a sorted string of hosts
	kubeCmd := fmt.Sprintf(
		"kubectl --v=%d -n orch-boots get cert tls-boots -o json", verboseLevel)
	hosts, err := Gen{}.dnsNames(kubeCmd)
	if err != nil {
		return nil, err
	}
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no ingress entries found for BIOS Traefik hosts, is orchestrator deployed?")
	}

	// Sort to make it easier for humans to search
	sort.Strings(hosts)
	return hosts, err
}

func (Gen) hostfile(ip string, addComment bool) error {
	hosts, err := (Gen{}).kubeDnslookupDockerInternal()
	if err != nil {
		return err
	}
	if addComment {
		fmt.Println("### BEGIN ORCH DEVELOPMENT HOSTS")
	}

	// Track hosts that were already printed
	uniqueHosts := map[string]struct{}{}

	// Print host if unique
	for _, host := range hosts {
		if _, ok := uniqueHosts[host]; !ok {
			fmt.Printf("%s %s\n", ip, host)
			uniqueHosts[host] = struct{}{}
		}
	}

	if addComment {
		fmt.Println("### END ORCH DEVELOPMENT HOSTS")
	}

	return nil
}

func (Gen) BiosTraefikhostfile(ip string, addComment bool) error {
	hosts, err := (Gen{}).kubeBiosDnslookupDockerInternal()
	if err != nil {
		return err
	}
	if addComment {
		fmt.Println("### BEGIN BIOS TRAEFIK HOSTS")
	}

	// Track hosts that were already printed
	uniqueHosts := map[string]struct{}{}

	// Print host if unique
	if err != nil {
		return err
	}
	for _, host := range hosts {
		if _, ok := uniqueHosts[host]; !ok {
			fmt.Printf("%s %s\n", ip, host)
			uniqueHosts[host] = struct{}{}
		}
	}

	if addComment {
		fmt.Println("### END BIOS TRAEFIK HOSTS")
	}

	return nil
}

func (Gen) GenericHostfile(ip string, host string, addComment bool) error {
	if addComment {
		fmt.Printf("### BEGIN %s HOSTS\n", strings.ToUpper(host))
	}
	fmt.Printf("%s %s.%s\n", ip, host, serviceDomain)
	if addComment {
		fmt.Printf("### END %s HOSTS\n", strings.ToUpper(host))
	}
	return nil
}

func (Gen) kubeDnslookupDockerInternalSRE() ([]string, error) {
	kubeCmd := fmt.Sprintf("kubectl -v %d -n orch-sre get cert kubernetes-docker-internal -o json",
		verboseLevel)
	hosts, err := Gen{}.dnsNames(kubeCmd)
	if err != nil {
		return nil, err
	}
	if len(hosts) != 1 {
		return nil, fmt.Errorf("expecting one SRE endpoint deployed with LoadBalancer svc, got %d", len(hosts))
	}
	return hosts, err
}

func (Gen) dnsNamesConfigMap(kubeCmd string) ([]string, error) {
	data, err := script.NewPipe().Exec(kubeCmd).String()
	if err != nil {
		return nil, fmt.Errorf("error getting k8s dns Names from configmap: %w", err)
	}
	hosts, err := script.Echo(data).JQ(".data.dnsNames").Replace(`"-`, "").String()
	if err != nil {
		return nil, fmt.Errorf("error executing Echo: %w", err)
	}
	hostsSplit := strings.Split(strings.TrimSpace(hosts), `\n- `)
	s := make([]string, 0)
	for _, host := range hostsSplit {
		if host != "" {
			s = append(s, strings.ReplaceAll(host, `\n"`, ""))
		}
	}
	return s, nil
}

func (Gen) dnsNames(kubeCmd string) ([]string, error) {
	data, err := script.NewPipe().Exec(kubeCmd).String()
	if err != nil {
		return nil, fmt.Errorf("error getting k8s dns Names from cert: %w", err)
	}
	hosts, err := script.Echo(data).JQ(" .spec.dnsNames | .[]").Replace(`"`, "").Slice()
	if err != nil {
		return nil, fmt.Errorf("error executing Echo: %w", err)
	}
	s := make([]string, 0)
	for _, host := range hosts {
		if host != "" {
			s = append(s, host)
		}
	}
	return s, nil
}

// Generates the full lets encrypt ca bundle, including the LE root. This is only needed by robotframework tests.
// Needed due to staging LE certificates not being trusted by the browser.
// The underlying requests library that robot framework uses reads REQUESTS_CA_BUNDLE , which requires the full chain of certificates
func (Gen) orchCABundle(filePath string) error {
	// Retrieve the lets encrypt staging root
	cmd := "curl -s https://letsencrypt.org/certs/staging/letsencrypt-stg-root-x1.pem"
	stagingRootCert, err := script.NewPipe().Exec(cmd).String()
	if err != nil {
		return err
	}

	err = Gen{}.orchCA(filePath)
	if err != nil {
		panic(err)
	}
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	// Write the string to the file
	_, err = file.WriteString(stagingRootCert)
	if err != nil {
		panic(err)
	}

	fmt.Printf(`Retrieved CA certificate:
Appended LE Root certificate and wrote Bundle to %s
export filepath to REQUESTS_CA_BUNDLE if using for robot framework test with staging LE certificate.
`,
		filePath,
	)
	return nil
}

// OrchCA Saves Orchestrators's CA certificate to `orch-ca.crt` so it can be imported to trust store for web access.
func (Gen) orchCA(filePath string) error {
	var certKey string
	kubeCmd := fmt.Sprintf("kubectl --v=%d get secret -n orch-gateway tls-orch -o json",
		verboseLevel)

	data, err := script.NewPipe().Exec(kubeCmd).String()
	if err != nil {
		return fmt.Errorf("get secret: %w: %s", err, data)
	}

	// if ca.crt key not available , then use the tls.crt
	// in the case that this is an autocert generation
	hasCAKey, err := script.Echo(data).JQ(".data | has(\"ca.crt\")").Replace("\n", "").String()
	if err != nil {
		return fmt.Errorf("check JSON for certificate: %w", err)
	}
	hasCAKey = strings.ReplaceAll(hasCAKey, "\n", "")

	if hasCAKey == "true" {
		// Self-signed certificate would have the ca.crt
		certKey = "ca.crt"
	} else {
		// Auto cert deployment would not have the ca.crt
		// but would have the full chain stored in tls.crt
		certKey = "tls.crt"
	}

	searchKey := fmt.Sprintf(`.data."%s"`, certKey)
	cert, err := script.Echo(data).JQ(searchKey).Replace(`"`, "").String()
	if err != nil {
		return fmt.Errorf("parse JSON for certificate: %w", err)
	}

	caCertBytes, err := base64.StdEncoding.DecodeString(cert)
	if err != nil {
		return fmt.Errorf("decode base64 certificate: %w", err)
	}

	if err := os.WriteFile(
		filePath,
		caCertBytes,
		os.ModePerm,
	); err != nil {
		return fmt.Errorf("write certificate file: %w", err)
	}

	fmt.Printf(`Retrieved CA certificate:
%s
Wrote CA certificate to %s ✍️
Add to your system trust store so your browser or client trusts the TLS certificate presented by Orchestrator services 🤔
`,
		string(caCertBytes),
		filePath,
	)
	return nil
}
