// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/bitfield/script"
	"github.com/magefile/mage/sh"
	"github.com/open-edge-platform/edge-manageability-framework/on-prem-installers/mage"
)

const keHeader = `
 _  ________   _____ _   _  _____ _______       _      _      ______ _____  
| |/ /  ____| |_   _| \ | |/ ____|__   __|/\   | |    | |    |  ____|  __ \ 
| ' /| |__      | | |  \| | (___    | |  /  \  | |    | |    | |__  | |__) |
|  < |  __|     | | | . | |\___ \   | | / /\ \ | |    | |    |  __| |  _  / 
| . \| |____   _| |_| |\  |____) |  | |/ ____ \| |____| |____| |____| | \ \ 
|_|\_\______| |_____|_| \_|_____/   |_/_/    \_\______|______|______|_|  \_\									   
`
const configHeader = `
 ____            _                    ____             __ _
/ ___| _   _ ___| |_ ___ _ __ ___    / ___|___  _ __  / _(_) __ _
\___ \| | | / __| __/ _ \ '_ ' _ \  | |   / _ \| '_ \| |_| |/ _' |
 ___) | |_| \__ \ ||  __/ | | | | | | |__| (_) | | | |  _| | (_| |
|____/ \__, |___/\__\___|_| |_| |_|  \____\___/|_| |_|_| |_|\__, |
       |___/                                                |___/
`

const (
	deploymentTimeoutEnv     = "DEPLOYMENT_TIMEOUT"
	deploymentDefaultTimeout = "3600s" // must be a valid duration string
)

var upgrade = flag.Bool("upgrade", false, "determine if KE should be upgraded or installed")

func main() {

	// Install RKE2 Cluster
	if err := os.Setenv("KUBECONFIG", fmt.Sprintf("/home/%s/.kube/config", os.Getenv("USER"))); err != nil {
		fmt.Printf("Error setting KUBECONFIG environment variable: %s\n", err)
		os.Exit(1)
	}

	if err := os.Setenv("INSTALLER_DEPLOY", "true"); err != nil {
		fmt.Printf("Error setting INSTALLER_DEPLOY environment variable: %s\n", err)
		os.Exit(1)
	}

	// Verify deployment timeout is appropriately set, else set a good default value for offline deployment --start
	timeoutStr := os.Getenv(deploymentTimeoutEnv)
	if timeoutStr == "" {
		if err := os.Setenv(deploymentTimeoutEnv, fmt.Sprintf("%v", deploymentDefaultTimeout)); err != nil {
			fmt.Printf("Error setting %v environment variable: %s\n", deploymentTimeoutEnv, err)
			os.Exit(1)
		}
	} else {
		_, err := time.ParseDuration(timeoutStr)
		if err != nil {
			fmt.Printf("deployment timeout must be a valid duration string: %v", err)
			os.Exit(1)
		}
	}
	// --end

	flag.Parse()
	if *upgrade {
		if err := (mage.Upgrade{}).Rke2Cluster(); err != nil {
			fmt.Printf("Error upgrading cluster: %s\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Install System Configurations
	if err := installSystemConfig(); err != nil {
		log.Fatal(err)
	}

	// Deploy Online OnPrem RKE2 cluster
	fmt.Print(keHeader)
	if err := (mage.Deploy{}).Rke2Cluster(); err != nil {
		fmt.Printf("Error deploying local cluster: %s\n", err)
		os.Exit(1)
	}
}

func installSystemConfig() error {
	fmt.Print(configHeader)

	if err := updateFanotifyFD("/etc/sysctl.conf"); err != nil {
		return err
	}

	if err := preInstallPkg(); err != nil {
		return err
	}

	hostpathDirs := []string{"/var/openebs/local"}
	if err := ensureHostpathDirectories(hostpathDirs); err != nil {
		return err
	}

	if err := configModules(); err != nil {
		return err
	}

	fmt.Println("OnPrem OS configure completed!")
	return nil
}

type BlockInfo struct {
	Name        string      `json:"name"`
	Size        int64       `json:"size"`
	Type        string      `json:"type"`
	MountPoints []string    `json:"mountpoints"`
	Children    []BlockInfo `json:"children,omitempty"`
}

func privateCmdExcute(cmdline string) error {
	cmd := exec.Command("/bin/sh", "-c", cmdline)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error executing %s commands: %w", cmdline, err)
	}
	return nil
}

// Enable kernel modules required for LV snapshots
func configModules() error {
	fmt.Println("config kernel modules...")

	mods, err := os.OpenFile("/etc/modules-load.d/lv-snapshots.conf", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer mods.Close()

	if _, err = mods.WriteString("dm-snapshot\ndm-mirror\n"); err != nil {
		return err
	}

	if err = sh.RunV("modprobe", "dm-snapshot"); err != nil {
		return err
	}
	if err = sh.RunV("modprobe", "dm-mirror"); err != nil {
		return err
	}

	return nil
}

func installYqTool(fileName string) error {
	cmdline := "curl https://github.com/mikefarah/yq/releases/latest -s -L -I -o /dev/null -w "
	pipe := script.NewPipe().Exec(cmdline + "'%{url_effective}'")
	out, err := pipe.ReplaceRegexp(regexp.MustCompile(".*/"), "").String()
	if err != nil {
		return err
	}
	version := strings.ReplaceAll(out, "\n", "")

	yqURL := fmt.Sprintf("https://github.com/mikefarah/yq/releases/download/%s/%s", version, fileName)

	headers := make(http.Header)
	headers.Set("User-Agent", "My-App/1.0")

	if _, err = url.Parse(yqURL); err != nil {
		return err
	}

	// Save the current working directory
	origDir, err := os.Getwd()
	if err != nil {
		return err
	}
	// Ensure we revert to the original directory before returning
	defer func() {
		_ = os.Chdir(origDir)
	}()

	// Check file is exist
	if err = os.Chdir("/tmp"); err != nil {
		return err
	}
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		fmt.Println("File does not exist")
	} else if err != nil {
		return err
	} else {
		// Delete file
		err := os.Remove(fileName)
		if err != nil {
			return err
		}
	}

	var cmdlines []string
	cmdline = fmt.Sprintf("curl -fsSL -o /tmp/%s %s", fileName, yqURL)
	fmt.Println(cmdline)
	cmdlines = append(cmdlines, cmdline)
	cmdline = fmt.Sprintf("tar xvf /tmp/%s -C /usr/local/bin", fileName)
	cmdlines = append(cmdlines, cmdline)
	cmdline = "mv /usr/local/bin/yq_linux_amd64 /usr/local/bin/yq"
	cmdlines = append(cmdlines, cmdline)
	cmdline = "chmod +x /usr/local/bin/yq"
	cmdlines = append(cmdlines, cmdline)

	for _, cmdline := range cmdlines {
		if err := privateCmdExcute(cmdline); err != nil {
			return err
		}
	}

	return nil
}

func installHelmTool(fileName string, version string) error {
	var cmdlines []string
	helmURL := "https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3"

	// Save the current working directory
	origDir, err := os.Getwd()
	if err != nil {
		return err
	}
	// Ensure we revert to the original directory before returning
	defer func() {
		_ = os.Chdir(origDir)
	}()

	// Change to /tmp
	if err := os.Chdir("/tmp"); err != nil {
		return err
	}

	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		fmt.Println("File does not exist")
	} else if err != nil {
		return err
	} else {
		// Delete file
		err := os.Remove(fileName)
		if err != nil {
			return err
		}
	}
	cmdline := fmt.Sprintf("curl -fsSL -o /tmp/%s %s", fileName, helmURL)
	fmt.Println(cmdline)
	cmdlines = append(cmdlines, cmdline)
	cmdline = fmt.Sprintf("chmod 700 /tmp/%s", fileName)
	cmdlines = append(cmdlines, cmdline)
	cmdline = fmt.Sprintf("/tmp/%s --version %s", fileName, version)
	cmdlines = append(cmdlines, cmdline)

	for _, cmdline := range cmdlines {
		if err := privateCmdExcute(cmdline); err != nil {
			return err
		}
	}

	return nil
}

func preInstallPkg() error {
	fmt.Println("Install dependency packages...")

	if err := installYqTool("yq_linux_amd64.tar.gz"); err != nil {
		return err
	}

	return installHelmTool("get_helm.sh", "v3.12.3")
}

func writeSysctlConfig(file *os.File, found1, found2, found3 bool) error {
	if !found1 {
		_, err := file.WriteString("fs.inotify.max_queued_events = 1048576\n")
		if err != nil {
			return fmt.Errorf("writing file: %w", err)
		}
	}
	if !found2 {
		_, err := file.WriteString("fs.inotify.max_user_instances = 1048576\n")
		if err != nil {
			return fmt.Errorf("writing file: %w", err)
		}
	}
	if !found3 {
		_, err := file.WriteString("fs.inotify.max_user_watches = 1048576\n")
		if err != nil {
			return fmt.Errorf("writing file: %w", err)
		}
	}
	cmdline := "sysctl -p"

	return privateCmdExcute(cmdline)
}

func updateFanotifyFD(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	found1 := false
	found2 := false
	found3 := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "#") {
			if strings.Contains(line, "fs.inotify.max_queued_events = 1048576") {
				found1 = true
			}
			if strings.Contains(line, "fs.inotify.max_user_instances = 1048576") {
				found2 = true
			}
			if strings.Contains(line, "fs.inotify.max_user_watches = 1048576") {
				found3 = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning file:%w", err)
	}

	return writeSysctlConfig(file, found1, found2, found3)
}

// Ensure necessary directories for Hostpath
func ensureHostpathDirectories(directories []string) error {
	for _, dir := range directories {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}
	}
	return nil
}
