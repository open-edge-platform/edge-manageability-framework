// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
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

func chownToCurrentUserRecursive(root string) error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, gid)
	})
}
