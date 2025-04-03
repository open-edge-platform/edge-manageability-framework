// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bitfield/script"
	"github.com/magefile/mage/sh"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-edge-platform/edge-manageability-framework/internal/retry"
	nexus_client "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/nexus-client"
)

const (
	TestOrg        = "e2eTestOrg"
	TestProject    = "e2eTestProj"
	TestUser       = "e2eTestUser"
	TenancyOrg     = "tenancyOrg"
	TenancyProject = "tenancyProject"
)

func (Test) golang() error {
	return sh.RunV("ginkgo", "-v", "-r", "--skip-package", "assets", "--label-filter=!orchestrator-integration && !tenancy && !stress-test && !fleet-management && !autocert && !cluster-orch-smoke-test && !cluster-orch-smoke-test-cleanup && !observability && !orchestrator-observability && !edgenode-observability && !observability-alerts && !sre-observability")
}

// prepare environment for e2e mage tests
// create the org TestOrg
// create the project TestProject
// create users TestUser-edge-op, TestUser-edge-man in the project
func (Test) createTestProject(ctx context.Context) error {
	var err error

	tu := TenantUtils{}

	// create testOrg if it does not exist
	err = tu.GetOrg(ctx, TestOrg)
	if nexus_client.IsNotFound(err) {
		err = tu.CreateOrg(ctx, TestOrg)
	}
	if err != nil {
		return fmt.Errorf("error creating org: %w", err)
	}

	// create testProject if it does not exist
	err = tu.GetProject(ctx, TestOrg, TestProject)
	if nexus_client.IsNotFound(err) {
		err = tu.CreateProjectInOrg(ctx, TestOrg, TestProject)
	}
	if err != nil {
		return fmt.Errorf("error creating project: %w", err)
	}

	// make sure it's ready
	// It's possible we failed out last time in GetProject, and the project never got
	// provisioned. If so, the roles won't be ready, and creating the user will fail
	_, err = tu.WaitUntilProjectReady(ctx, TestOrg, TestProject)
	if err != nil {
		return err
	}

	// get the project id
	projectId, err := GetProjectId(ctx, TestOrg, TestProject)
	if projectId == "" || err != nil {
		return fmt.Errorf("error retrieving project %s", TestProject)
	}

	// create the test users if they do not exist
	fmt.Println("Creating test users for CO")
	err = tu.CreateClusterOrchUsers(ctx, TestOrg, TestProject, TestUser)
	if err != nil && status.Code(err) != codes.AlreadyExists {
		return err
	}
	fmt.Println("Creating test users for Edge Infra Manager")
	err = tu.CreateEdgeInfraUsers(ctx, TestOrg, TestProject, TestUser)
	if err != nil && status.Code(err) != codes.AlreadyExists {
		return err
	}

	return nil
}

func (Test) e2eTenancyApiGw() error {
	return sh.RunV(
		"ginkgo",
		"-v",
		"-r",
		"-p",
		"--skip-package", "assets",
		"--label-filter=tenancy",
		filepath.Join("e2e-tests", "orchestrator"),
	)
}

func (Test) e2eObservability() error {
	return sh.RunV(
		"ginkgo",
		"-v",
		"-r",
		"-p",
		"--skip-package", "assets",
		"--label-filter=observability",
		filepath.Join("e2e-tests", "orchestrator"),
	)
}

func (Test) e2eOrchObservability() error {
	return sh.RunV(
		"ginkgo",
		"-v",
		"-r",
		"-p",
		"--skip-package", "assets",
		"--label-filter=orchestrator-observability",
		filepath.Join("e2e-tests", "orchestrator"),
	)
}

func (Test) e2eEnObservability() error {
	return sh.RunV(
		"ginkgo",
		"-v",
		"-r",
		"-p",
		"--skip-package", "assets",
		"--label-filter=edgenode-observability",
		filepath.Join("e2e-tests", "orchestrator"),
	)
}

func (Test) e2eAlertsObservability() error {
	return sh.RunV(
		"ginkgo",
		"-v",
		"-r",
		"-p",
		"--skip-package", "assets",
		"--label-filter=observability-alerts",
		filepath.Join("e2e-tests", "orchestrator"),
	)
}

func (Test) e2eSreObservabilityNoEnic() error {
	return sh.RunV(
		"ginkgo",
		"-v",
		"-r",
		"-p",
		"--skip-package", "assets",
		"--label-filter=sre-observability && enic-not-deployed",
		filepath.Join("e2e-tests", "orchestrator"),
	)
}

func (Test) e2eSreObservability() error {
	return sh.RunV(
		"ginkgo",
		"-v",
		"-r",
		"-p",
		"--skip-package", "assets",
		"--label-filter=sre-observability && enic-deployed",
		filepath.Join("e2e-tests", "orchestrator"),
	)
}

func (Test) tenancyTestsViaClient(ctx context.Context) error {
	/*  e2eTenancy test scenario :-
	1. Create Org
	2. Check if the OrgActiveWatcher is created and status is set to STATUS_INDICATION_IDLE
	3. Check if Org status is set to STATUS_INDICATION_IDLE
	4. Create Project under the org
	5. Check if 6/8 ProjectActiveWatchers are created and everyone's status should be set to STATUS_INDICATION_IDLE
	5.1. If timed out, check if Project status is not STATUS_INDICATION_IDLE and shows the underlying active watcher
	5.2 If every active watcher's status is STATUS_INDICATION_IDLE, validate if Project status is set to STATUS_INDICATION_IDLE
	6. Delete Project under the org
	7. Every ProjectActiveWatchers should be deleted? And project should be deleted
	8. Delete Org and OrgActiveWatcher should be deleted
	*/
	var err error

	err = createAndWaitForOrg(ctx)
	if err != nil {
		return err
	}
	err = createAndWaitForProject(ctx)
	if err != nil {
		return err
	}
	err = deleteProject(ctx)
	// Check if err is waiting status
	if err != nil && !strings.Contains(err.Error(), "status message is set to") {
		return err
	}
	err = deleteOrg(ctx)
	if err != nil && !strings.Contains(err.Error(), "waiting for watchers to be deleted with status message") {
		return err
	}
	return nil
}

func createAndWaitForOrg(ctx context.Context) error {
	var err error
	tu := TenantUtils{}
	// Create test Org
	fmt.Println("----Creating and waiting for Org watcher to be created----")
	err = tu.CreateOrg(ctx, TenancyOrg)
	if err != nil {
		return fmt.Errorf("error creating org: %w", err)
	}

	// Get testOrg
	err = tu.GetOrg(ctx, TenancyOrg)
	if nexus_client.IsNotFound(err) {
		return fmt.Errorf("error fetching org: %w", err)
	}
	if err != nil {
		return fmt.Errorf("error during org fetch: %w", err)
	}
	return nil
}

func deleteOrg(ctx context.Context) error {
	var err error
	tu := TenantUtils{}
	// Delete Org
	fmt.Println("----Deleting Org----")
	err = tu.DeleteOrg(ctx, TenancyOrg)
	if err != nil {
		if err.Error() == fmt.Sprintf("org %s deletion timed out", TenancyOrg) {
			return tu.GetOrg(ctx, TenancyOrg)
		}
		return err
	}
	fmt.Println("----Deleted Org----")
	return nil
}

func createAndWaitForProject(ctx context.Context) error {
	var err error
	tu := TenantUtils{}
	// create testProject
	fmt.Println("----Creating Project and Waiting for UID to be created----")
	err = tu.CreateProjectInOrg(ctx, TenancyOrg, TenancyProject)
	if err != nil {
		return fmt.Errorf("error creating project: %w", err)
	}
	// wait until project activewatchers status set to idle
	fmt.Println("----Waiting for all project watchers to be ready----")
	statusMsg, err := tu.WaitUntilProjectWatchersReady(ctx, TenancyOrg, TenancyProject)
	if err != nil {
		return err
	}
	if statusMsg != "all watchers ready and in idle state" {
		return fmt.Errorf("failed: %v", statusMsg)
	}
	// wait until project status set to ready
	fmt.Println("----Checking if Project status is set to Ready----")
	_, err = tu.WaitUntilProjectReady(ctx, TenancyOrg, TenancyProject)
	if err != nil {
		return err
	}
	// Get testProject
	err = tu.GetProject(ctx, TenancyOrg, TenancyProject)
	if nexus_client.IsNotFound(err) {
		return fmt.Errorf("error fetching project: %w", err)
	}
	if err != nil {
		return err
	}
	// get the project id
	//projectId, err := GetProjectId(ctx, TestOrg, TestProject)
	//if projectId == "" || err != nil {
	//	return fmt.Errorf("error retrieving project %s", TestProject)
	//}
	return nil
}

func deleteProject(ctx context.Context) error {
	var err error
	tu := TenantUtils{}
	// Delete Project
	fmt.Println("----Deleting Project---")
	err = tu.DeleteProject(ctx, TenancyOrg, TenancyProject)
	if err != nil {
		if err.Error() == fmt.Sprintf("project %s deletion timed out", TenancyProject) {
			return tu.GetProject(ctx, TenancyOrg, TenancyProject)
		}
		return err
	}
	return nil
}

func (Test) e2e() error {
	return sh.RunV(
		"ginkgo",
		"-v",
		"-r",
		"-p",
		"--skip-package", "assets",
		"-randomize-all",
		"-randomize-suites",
		"--label-filter=orchestrator-integration",
		filepath.Join("e2e-tests", "orchestrator"),
	)
}

// Test end-to-end functionality of onboarding an Edge Node. These tests cannot be executed concurrently due to issues
// with the underlying virtual Edge Node provisioning logic.
func (Test) Onboarding() error {
	return sh.RunV(
		"ginkgo",
		"-v",
		filepath.Join("e2e-tests", "onboarding"),
	)
}

func (Test) stress() error {
	return sh.RunV(
		"ginkgo",
		"-v",
		"-r",
		"-p",
		"--fail-fast",
		"--race",
		"-randomize-all",
		"-randomize-suites",
		"--label-filter=!orchestrator-integration && stress-test",
		filepath.Join("e2e-tests", "orchestrator"),
	)
}

// Test Runs cluster orch smoke test by creating locations, configuring host, creating a cluster and then finally cleanup
func (Test) clusterOrchSmoke() error {
	return sh.RunV(
		"ginkgo",
		"-v",
		"-r",
		"--fail-fast",
		"--race",
		"--label-filter=cluster-orch-smoke-test",
		filepath.Join("e2e-tests", "orchestrator"),
	)
}

// Test end-to-end functionality of Orchestrator deployed with autocert.
func (Test) e2eAutocert() error {
	return sh.RunV(
		"ginkgo",
		"-v",
		"-r",
		"-p",
		"--fail-fast",
		"--race",
		"-randomize-all",
		"-randomize-suites",
		"--label-filter=autocert",
		filepath.Join("e2e-tests", "orchestrator/autocert_test"),
	)
}

// Test end-to-end functionality for a specific label.
func (Test) e2eByLabel(label string) error {
	return sh.RunV(
		"ginkgo",
		"-v",
		"-r",
		"-p",
		"--fail-fast",
		"--race",
		"-randomize-all",
		"-randomize-suites",
		fmt.Sprintf("--label-filter=%s", label),
		filepath.Join("e2e-tests", "orchestrator"),
	)
}

func (Test) pods() error {
	timeout, err := parseDeploymentTimeout()
	if err != nil {
		return err
	}
	expireTime := time.Now().Add(timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := retry.UntilItSucceeds(
		ctx,
		func() error {
			fmt.Println("~~~~~~~~~~")
			// Filter and Count all pods that are Ready
			data, err := script.NewPipe().
				Exec("kubectl get pods -A -o json").
				JQ(`.items[] | select(.status.phase = "Ready" or ([ .status.conditions[] |
				select(.type == "Ready") ] | length ) == 1 ) | .metadata.namespace + "/" + .metadata.name`).
				String()
			if err != nil {
				return err
			}
			totalReadyPods, err := script.Echo(data).CountLines()
			if err != nil {
				return err
			}

			// Count all pods in the system
			data, err = script.NewPipe().
				Exec("kubectl get pods -A").
				Reject("NAMESPACE").
				String()
			if err != nil {
				return err
			}
			totalPods, err := script.Echo(data).CountLines()
			if err != nil {
				return err
			}

			fmt.Println("totalPods   ", totalPods)
			fmt.Println("totalReadyPods ", totalReadyPods)
			if totalPods == 0 {
				fmt.Println("Time remaining ⏰: ", time.Until(expireTime))
				return fmt.Errorf("pods have not yet started")
			}
			// Wait until the total Ready pods is equal to the total number of pods
			if totalPods != totalReadyPods {
				fmt.Println("Time remaining ⏰: ", time.Until(expireTime))
				return fmt.Errorf("not all pods ready yet: %d", totalPods-totalReadyPods)
			}
			return nil
		},
		5*time.Second,
	); err != nil {
		return fmt.Errorf("test failed: %w ❌", err)
	}

	fmt.Println("Pod Test completed ✅")

	return nil
}

func (Test) flexCore() error {
	cmd := "kind/flexcore-test.sh"
	if _, err := script.Exec(cmd).Stdout(); err != nil {
		return err
	}
	return nil
}

func (Test) deployment() error { //nolint: cyclop
	type status struct {
		ReadyReplicas int `json:"readyReplicas"`
		Replicas      int `json:"replicas"`
	}

	timeout, err := parseDeploymentTimeout()
	if err != nil {
		return err
	}

	expireTime := time.Now().Add(timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := retry.UntilItSucceeds(
		ctx,
		func() error {
			fmt.Println("~~~~~~~~~~")
			data, err := script.NewPipe().
				Exec("kubectl get deployments -A -o json").
				JQ(".items | .[] | .status | {replicas, readyReplicas}").
				String()
			if err != nil {
				return err
			}
			fail, err := script.Echo(data).
				FilterScan(
					func(line string, w io.Writer) {
						var s status
						if err := json.Unmarshal([]byte(line), &s); err != nil {
							panic(err)
						}
						if s.ReadyReplicas != s.Replicas {
							if _, err := w.Write([]byte(fmt.Sprintf("%s\n", line))); err != nil {
								panic(err)
							}
						}
					}).
				CountLines()
			if err != nil {
				return err
			}
			total, err := script.Echo(data).CountLines()
			if err != nil {
				return err
			}
			fmt.Println("ok   ", total-fail)
			fmt.Println("fail ", fail)
			fmt.Println("----------")
			fmt.Println("total", total)
			fmt.Println("----------")
			if total == 0 {
				fmt.Println("Time remaining ⏰: ", time.Until(expireTime))
				return fmt.Errorf("deployments have not started yet")
			}
			if fail != 0 {
				fmt.Println("Time remaining ⏰: ", time.Until(expireTime))
				return fmt.Errorf("remaining failed deployments: %d", fail)
			}
			return nil
		},
		5*time.Second,
	); err != nil {
		return fmt.Errorf("Test failed: %w ❌", err)
	}

	fmt.Println("Deployment Test completed ✅")

	return nil
}

func parseDeploymentTimeout() (time.Duration, error) {
	// Use timeout value set in env variable if available, else defaults
	timeoutStr := os.Getenv(deploymentTimeoutEnv)
	var timeout time.Duration
	var err error
	if timeoutStr == "" {
		timeout, err = time.ParseDuration(defaultDeploymentTimeout)
	} else {
		timeout, err = time.ParseDuration(timeoutStr)
	}
	return timeout, err
}

type kyvernoPolicyReport struct {
	Scope struct {
		Kind      string `json:"kind"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"scope"`

	Results []struct {
		Policy  string `json:"policy"`
		Result  string `json:"result"`
		Rule    string `json:"rule"`
		Message string `json:"message"`
	}

	Summary struct {
		Fail uint64 `json:"fail"`
	} `json:"summary"`
}

type policyReport struct {
	Items []kyvernoPolicyReport `json:"items"`
}

// Tests if any Kubernetes cluster resources are in violation of a cluster policy. If a resource is in violation, the
// resource metadata and specific policy violations are printed to stdout. If there are no policy violations, this
// target returns nil.
func (Test) PolicyCompliance(ctx context.Context) error {
	var sumPolicyRuleViolations uint64
	report, err := getPolicyReport("policyreports", ctx)
	if err != nil {
		fmt.Printf("uanble to get %s: %s", "policyreports", err.Error())
	}
	fmt.Println("============== PolicyReport ===========================")
	sumPolicyRuleViolations += printPolicyReport(report, true)

	cpReport, err := getPolicyReport("clusterpolicyreport", ctx)
	if err != nil {
		fmt.Printf("uanble to get %s: %s", "clusterpolicyreport", err.Error())
	}
	fmt.Println("============== ClusterPolicyReport ====================")
	sumPolicyRuleViolations += printPolicyReport(cpReport, true)

	if sumPolicyRuleViolations > 0 {
		return fmt.Errorf("FAIL - expected total policy rule violations to be 0, got %d ❌", sumPolicyRuleViolations)
	}
	fmt.Println("PASSED - no policy rule violations ✅")

	return nil
}

func getPolicyReport(policyType string, ctx context.Context) (*policyReport, error) {
	bytes, err := exec.CommandContext(
		ctx,
		"kubectl",
		"get",
		policyType,
		"-A",
		"-o", "json",
	).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("get policy reports: %w: %s", err, string(bytes))
	}

	output := &policyReport{
		Items: make([]kyvernoPolicyReport, 0),
	}
	if err := json.Unmarshal(bytes, output); err != nil {
		return nil, fmt.Errorf("parse policy reports: %w", err)
	}
	return output, nil
}

func printPolicyReport(pReport *policyReport, isClusterPolicy bool) uint64 {
	// Filter reports so only reports with failures remain and sum total rule violations.
	var sumPolicyRuleViolations uint64
	for _, report := range pReport.Items {
		if report.Summary.Fail > 0 {
			sumPolicyRuleViolations += report.Summary.Fail

			// Print the report to make it easier to read
			if isClusterPolicy {
				fmt.Printf(
					"Kind: %s, Name: %s\n",
					report.Scope.Kind,
					report.Scope.Name,
				)
			} else {
				fmt.Printf(
					"Namespace: %s, Kind: %s, Name: %s\n",
					report.Scope.Namespace,
					report.Scope.Kind,
					report.Scope.Name,
				)
			}
			for _, result := range report.Results {
				if result.Result == "fail" {
					fmt.Printf("FAIL Policy: %s, Rule: %s, Message: %s \n", result.Policy, result.Rule, result.Message)
				}
			}
			fmt.Println("-------------------------------------------------------")
		}
	}
	if sumPolicyRuleViolations == 0 {
		fmt.Println("    PASSED - no policy rule violations ✅")
	}

	return sumPolicyRuleViolations
}

// Tests if any Kubernetes pod violates image pull policy.
func (Test) ImagePullPolicyCompliance(ctx context.Context) error {
	violations, err := script.NewPipe().
		Exec("kubectl get pods -A -o json").
		JQ(`.items[] | . as $pod | .spec.containers[] | select(.imagePullPolicy=="Always") | "\($pod.metadata.namespace) \($pod.metadata.name) \(.name)"`).
		Exec("column -t").String()
	if err != nil {
		return fmt.Errorf("FAIL - unable to get pods: %w ❌", err)
	}

	lines, err := script.Echo(violations).CountLines()
	if err != nil {
		return fmt.Errorf("FAIL - unable to count lines: %w ❌", err)
	}

	if lines != 0 {
		fmt.Print(violations)
		return fmt.Errorf("FAIL - expected total policy rule violations to be 0, got %d ❌", lines)
	}
	fmt.Println("PASSED - no policy rule violations ✅")
	return nil
}

const registryCheckTimeout = 15 * time.Second

// Return nil if all charts necessary to deploy Orchestrator are available in the Release service.
// TODO: Only cloud orch is tested right now. Update for on-prem or check whether this is good enough for on-prem.
func (Test) ChartsAvailableOnReleaseService(ctx context.Context) error {
	// Get all charts for orchestrator that should be in the Release service
	manifest, err := getManifest()
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	filteredComponents := []ComponentDetails{}

	// Only test charts that are first party
	for _, component := range manifest.Components {
		if strings.Contains(component.Repo, PublicTiberRegistryRepoURL+"/"+IntelTiberRepository) {
			filteredComponents = append(filteredComponents, component)
		}
	}

	// Keep track of what charts are available. If true, it is available
	status := map[string]bool{}

	// If any chart is unavailable, fail the test
	fail := false

	// Guards status map and fail
	mu := &sync.Mutex{}

	eg := &errgroup.Group{}

	eg.SetLimit(runtime.NumCPU())

	// For each chart, query registry and see if it exists
	for _, component := range filteredComponents {
		eg.Go(func() error {
			chart := fmt.Sprintf(
				"%s/%s",
				component.Repo,
				component.Chart,
			)

			fmt.Println("Checking: ", chart)

			ctx, cancel := context.WithTimeout(ctx, registryCheckTimeout)
			defer cancel()

			statusKey := chart + " Version " + component.Version

			if stdouterr, err := exec.CommandContext(
				ctx,
				"helm",
				"show",
				"chart",
				"--version", component.Version,
				chart,
			).CombinedOutput(); err != nil {
				if strings.Contains(string(stdouterr), "not found") {
					mu.Lock()
					status[statusKey] = false
					fail = true
					mu.Unlock()
					return nil
				}
				return fmt.Errorf("show chart: %s: %w", string(stdouterr), err)
			}

			mu.Lock()
			status[statusKey] = true
			mu.Unlock()

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("check charts: %w", err)
	}

	fmt.Println("Charts that are not available:")

	count := 0
	for chart, available := range status {
		if !available {
			fmt.Printf("Chart: %s\n", chart)
			count++
		}
	}

	fmt.Println("-------------------------------------------------------------")

	if fail {
		return fmt.Errorf(
			"FAIL - expected all charts to be available on the Release service, but %d were not ❌",
			count,
		)
	}

	fmt.Println("PASSED - all charts are available on the Release service ✅")
	return nil
}

// Return nil if all containers necessary to deploy Orchestrator are available in the Release service.
// TODO: Only cloud orch is tested right now. Update for on-prem or check whether this is good enough for on-prem.
func (Test) ContainersAvailableOnReleaseService(ctx context.Context, firstPartyOnly bool) error {
	// Get all container images for orchestrator that should be in the Release service
	imagesList, _, err := getImageManifest()
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	// Some images are only available post-merge into the main branch and they should be ignored
	ignored := []string{"orchestrator-installer-cloudfull"}

	if firstPartyOnly {
		// Only test images that are first party
		for _, image := range imagesList {
			if !strings.Contains(image, IntelTiberRegistryRepoURL+"/"+IntelTiberRepository) {
				ignored = append(ignored, image)
			}
		}
	}

	// Keep track of what images are available. If true, it is available
	status := map[string]bool{}

	// If any image is unavailable, fail the test
	fail := false

	// Guards status map and fail
	mu := &sync.Mutex{}

	eg := &errgroup.Group{}

	eg.SetLimit(runtime.NumCPU())

	// For each image, query registry and see if it exists
	for _, image := range imagesList {
		eg.Go(func() error {
			if containsElementWithSubstring(ignored, image) {
				return nil
			}

			fmt.Println("Checking: ", image)

			ctx, cancel := context.WithTimeout(ctx, registryCheckTimeout)
			defer cancel()

			if stdouterr, err := exec.CommandContext(
				ctx,
				"docker",
				"manifest",
				"inspect",
				image,
			).CombinedOutput(); err != nil {
				if bytes.Contains(stdouterr, []byte("no such manifest")) {
					mu.Lock()
					status[image] = false
					fail = true
					mu.Unlock()
					return nil
				}
				return fmt.Errorf("inspect image: %s: %w", string(stdouterr), err)
			}

			mu.Lock()
			status[image] = true
			mu.Unlock()

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("check images: %w", err)
	}

	fmt.Println("Images that are not available:")

	count := 0
	for image, available := range status {
		if !available {
			fmt.Printf("Image: %s\n", image)
			count++
		}
	}

	fmt.Println("-------------------------------------------------------------")

	if fail {
		return fmt.Errorf(
			"FAIL - expected all images to be available on the Release service, but %d images were not ❌",
			count,
		)
	}

	fmt.Println("PASSED - all images are available on the Release service ✅")
	return nil
}

func containsElementWithSubstring(list []string, s string) bool {
	for _, element := range list {
		if strings.Contains(s, element) {
			return true
		}
	}
	return false
}
