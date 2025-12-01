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

	catalogloader "github.com/open-edge-platform/orch-library/go/pkg/loader"
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

	err = UploadFiles(paths, serviceDomain, orchProject, orchUser, orchPass)
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
