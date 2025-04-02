// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package mage

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	"github.com/bitfield/script"
	catalogloader "github.com/open-edge-platform/orch-library/tree/4a79080b16bf098e60ce7a2dbda57e99328e5774/go/pkg/loader"
)

func (App) upload() error {
	paths := []string{
		"e2e-tests/samples/00-common",
		"e2e-tests/samples/10-applications",
		"e2e-tests/samples/20-deployment-packages",
	}

	orchProject := defaultProject
	if orchProjectEnv := os.Getenv("ORCH_PROJECT"); orchProjectEnv != "" {
		orchProject = orchProjectEnv
	}

	// todo: remove hardcode
	orchUser := "sample-project-edge-mgr"
	if orchUserEnv := os.Getenv("ORCH_USER"); orchUserEnv != "" {
		orchUser = orchUserEnv
	}

	orchPass, err := GetDefaultOrchPassword()
	if err != nil {
		return err
	}
	if orchPassEnv := os.Getenv("ORCH_PASS"); orchPassEnv != "" {
		orchPass = orchPassEnv
	}

	err = UploadFiles(paths, defaultClusterDomain, orchProject, orchUser, orchPass)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Println("Apps Uploaded 😊")
	return nil
}

func UploadFiles(paths []string, domain string, projectName string, edgeInfraUser string, orchPass string) error {
	apiBaseURL := "https://api." + domain

	var tlsConfig *tls.Config
	cli := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: tlsConfig,
		},
	}

	loader := catalogloader.NewLoader(apiBaseURL, projectName)
	token, err := GetApiToken(cli, edgeInfraUser, orchPass)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	err = loader.LoadResources(context.Background(), *token, paths)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func (App) wordpress() error {
	if _, err := script.Exec("kind/wordpress-example.sh 0.1.0").Stdout(); err != nil {
		return err
	}
	return nil
}

func (App) wordpressFromPrivateRegistry() error {
	if _, err := script.Exec("kind/wordpress-example.sh 0.1.1").Stdout(); err != nil {
		return err
	}
	return nil
}

func (App) iperfWebVM() error {
	if _, err := script.Exec(fmt.Sprintf("kind/iperf-web-vm-example.sh 1.0.0 %s", serviceDomain)).Stdout(); err != nil {
		return err
	}
	return nil
}

func (App) nginx() error {
	if _, err := script.Exec(fmt.Sprintf("kind/nginx-scale-test.sh 0.1.0 1 0 %s", serviceDomain)).Stdout(); err != nil {
		return err
	}
	return nil
}
