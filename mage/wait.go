// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// appDeployDefaultMaxDuration is the default max duration to wait for an application to finish progressing to the
// synced and healthy state before considering the application deployment a failure.
var appDeployDefaultMaxDuration = 60 * time.Minute

// appDeployMaxDurationOverrides is the custom max duration for a specific application deployment. If an override is not
// specified for an application, the appDeployDefaultMaxDuration value is used.
var appDeployMaxDurationOverrides = map[string]time.Duration{
	"istiod":                     30 * time.Minute,
	"kiali":                      35 * time.Minute,
	"kube-prometheus-stack":      25 * time.Minute,
	"kyverno":                    15 * time.Minute,
	"mimir":                      20 * time.Minute,
	"orchestrator-observability": 40 * time.Minute,
	"postgresql":                 30 * time.Minute,
	"edgenode-observability":     40 * time.Minute,
	"root-app":                   120 * time.Minute, // Represents the full Orchestrator deployment
	"secrets-config":             90 * time.Minute,
	"sre-exporter":               20 * time.Minute,
	"traefik":                    20 * time.Minute,
}

// Blocks until the applications in the Orchestrator deployment are synced and healthy. If a particular application
// deployment does not enter the synced and healthy state after the appDeployDefaultMaxDuration, the overall
// deployment would be considered a failure and this method will return an error.
func (Deploy) WaitUntilComplete(ctx context.Context) error {
	for {
		err := checkApps(ctx)

		switch {
		case errors.Is(err, errAppDeployExceededMaxDuration):
			return err

		case errors.Is(err, errOrchNotReady):
			// expected so no-op

		case err != nil:
			fmt.Printf("Error: %s, will retry in 10 seconds\n", err)

		case err == nil:
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-time.After(10 * time.Second):
		}
	}
}

var (
	errOrchNotReady                 = errors.New("orchestrator is not ready")
	errAppDeployExceededMaxDuration = errors.New("application deployment time exceeded max duration")
)

type argoCDApp struct {
	Metadata struct {
		Name              string    `json:"name"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
	} `json:"metadata"`

	Status struct {
		Health struct {
			Status string `json:"status"`
		} `json:"health"`

		Sync struct {
			Status string `json:"status"`
		} `json:"sync"`

		OperationState struct {
			FinishedAt time.Time `json:"FinishedAt"`
		} `json:"operationState"`

		ReconciledAt time.Time `json:"reconciledAt"`
	} `json:"status"`
}

func checkApps(ctx context.Context) error {
	// Clear the terminal to make it look live. Only works on *nix systems.
	if runtime.GOOS == "linux" {
		fmt.Println("\033[2J")
	}

	execCmd := exec.CommandContext(ctx, "kubectl", "get", "application", "-A", "-o", "json")

	var out bytes.Buffer
	var stderr bytes.Buffer
	execCmd.Stdout = &out
	execCmd.Stderr = &stderr

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("error argocd applications: %w, stderr: %s", err, stderr.String())
	}

	var output struct {
		Items []argoCDApp `json:"items"`
	}

	if err := json.Unmarshal(out.Bytes(), &output); err != nil {
		return fmt.Errorf("unmarshal applications: %w", err)
	}
	if len(output.Items) == 0 {
		return fmt.Errorf("got zero applications")
	}

	var notReady []argoCDApp
	for _, app := range output.Items {
		// Skip if app deployment hasn't started
		if app.Metadata.CreationTimestamp.IsZero() {
			continue
		}

		// Skip if app deployment completed
		if app.Status.Sync.Status == "Synced" && app.Status.Health.Status == "Healthy" {
			continue
		}

		notReady = append(notReady, app)

		// Get the specific app override if it exists
		deployMax, ok := appDeployMaxDurationOverrides[app.Metadata.Name]
		if !ok {
			deployMax = appDeployDefaultMaxDuration
		}

		since := app.Metadata.CreationTimestamp
		if !app.Status.ReconciledAt.IsZero() && app.Status.Health.Status != "Degraded" {
			since = app.Status.ReconciledAt
		}
		if time.Since(since) > deployMax {
			printAppDeploymentsInProgess(output.Items)
			return fmt.Errorf(
				"application %s exceeded %s max duration: %w",
				app.Metadata.Name,
				deployMax,
				errAppDeployExceededMaxDuration,
			)
		}
	}

	if len(notReady) != 0 {
		printAppDeploymentsInProgess(notReady)
		fmt.Println("Applications not synced or healthy, will check again 10 seconds 🟡")
		return errOrchNotReady
	}

	printAppDeploymentsInProgess(output.Items)

	fmt.Println("All applications synced and healthy. Orchestrator is ready 🟢")

	return nil
}

func printAppDeploymentsInProgess(apps []argoCDApp) {
	fmt.Println("---------------------Applications (In progress)----------------------------")
	for _, app := range apps {
		var total time.Duration

		switch {
		case app.Metadata.CreationTimestamp.IsZero():
			// App has not been started, display zero

		case app.Status.Sync.Status == "Synced" && app.Status.Health.Status == "Healthy":
			// App is synced and healthy, total time is duration since it was updated until the time ArgoCD detected
			// that it finished
			total = app.Status.OperationState.FinishedAt.Sub(app.Metadata.CreationTimestamp)

		case app.Status.ReconciledAt.IsZero() || app.Status.Health.Status == "Degraded":
			// App is not synced and healthy and not reconciled, total time is duration since created
			total = time.Since(app.Metadata.CreationTimestamp)

		default:
			// App is not synced and healthy, total time is duration since updated
			total = time.Since(app.Status.ReconciledAt)
		}

		fmt.Printf(
			"Name: %s Sync: %s Health: %s Total: %s\n",
			app.Metadata.Name,
			app.Status.Sync.Status,
			app.Status.Health.Status,
			total.Truncate(time.Second),
		)
	}
	fmt.Println("---------------------------------------------------------------------------")
}
