// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package common

import (
    "fmt"
    "os/exec"
	"time"
)

func WaitForNamespaceCreation(namespace string) error {
	for {
		cmd := exec.Command("kubectl", "get", "ns", namespace, "-o", "jsonpath={.status.phase}")
		out, err := cmd.Output()
		if err != nil {
			return err
		}
		if string(out) == "Active" {
			break
		}
		fmt.Printf("%s\n", string(out))
		fmt.Printf("Waiting for namespace %s to be created...\n", namespace)
		time.Sleep(5 * time.Second)
	}
	return nil
}