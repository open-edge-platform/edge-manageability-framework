// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bitfield/script"
)

const (
	argoRootAppPath      = "argocd/root-app/Chart.yaml"
	argoApplicationsPath = "argocd/applications/Chart.yaml"
)

func getVersionFromFile() (string, error) {
	v, err := os.ReadFile("VERSION")
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(v))
	version = strings.ReplaceAll(version, "\n", "_")
	version = strings.ReplaceAll(version, " ", "")
	return version, nil
}

func getVersionFromChart(chartYaml string) (string, error) {
	cmd := fmt.Sprintf("cat %s | yq .version", chartYaml)
	v, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(v))
	return version, nil
}

func getArgoCdAppsVersion() (string, error) {
	return getVersionFromChart(argoApplicationsPath)
}

func getArgoCdRootVersion() (string, error) {
	return getVersionFromChart(argoRootAppPath)
}

func setArgoCdAppsVersion(version string) error {
	return setVersionToChart(argoApplicationsPath, version)
}

func setArgoCdRootVersion(version string) error {
	return setVersionToChart(argoRootAppPath, version)
}

func setVersionToChart(chartYaml string, version string) error {
	cmd := fmt.Sprintf("yq e -i '.version = \"%s\"' %s", version, chartYaml)
	_, err := script.Exec(cmd).Stdout()
	if err != nil {
		return err
	}
	cmd = fmt.Sprintf("yq e -i '.appVersion = \"%s\"' %s", version, chartYaml)
	_, err = script.Exec(cmd).Stdout()
	if err != nil {
		return err
	}
	return nil
}

func (v Version) checkVersion() error {
	const fixMsg = "Please run 'mage version:setVersion' to fix the issue."
	version, err := getVersionFromFile()
	if err != nil {
		return err
	}
	appsVersion, err := getArgoCdAppsVersion()
	if err != nil {
		return err
	}

	rootVersion, err := getArgoCdRootVersion()
	if err != nil {
		return err
	}
	if appsVersion != version {
		return fmt.Errorf("VERSION (%s) and %s (%s) don't match. %s", version, argoApplicationsPath, appsVersion, fixMsg)
	}
	if rootVersion != version {
		return fmt.Errorf("VERSION (%s) and %s (%s) don't match. %s", version, argoRootAppPath, rootVersion, fixMsg)
	}
	fmt.Println("All versions match")
	return nil
}

func (v Version) setVersion() error {
	version, err := getVersionFromFile()
	if err != nil {
		return err
	}
	if err = setArgoCdAppsVersion(version); err != nil {
		return err
	}

	if err = setArgoCdRootVersion(version); err != nil {
		return err
	}
	return nil
}
