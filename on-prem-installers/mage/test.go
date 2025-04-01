// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bitfield/script"

	"github.com/open-edge-platform/edge-manageability-framework/internal/retry"
)

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

	// Include time spent on waiting for pods in deploymentTimeout
	timeRemaining := fmt.Sprintf("%ds", int(time.Until(expireTime).Seconds()))
	if err := setDeploymentTimeout(timeRemaining); err != nil {
		return err
	}

	fmt.Println("Pod Test completed ✅")

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

	// Include time spent on waiting for deployments in deploymentTimeout
	timeRemaining := fmt.Sprintf("%ds", int(time.Until(expireTime).Seconds()))
	if err := setDeploymentTimeout(timeRemaining); err != nil {
		return err
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

func setDeploymentTimeout(timeoutStr string) error {
	if _, err := time.ParseDuration(timeoutStr); err != nil {
		return err
	}

	if err := os.Setenv(deploymentTimeoutEnv, timeoutStr); err != nil {
		return err
	}

	return nil
}
