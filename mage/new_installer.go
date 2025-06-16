// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

type NewInstaller mg.Namespace

var (
	rootDir        = "new-installer"
	buildDir       = "_build"
	coverProfile   = "coverprofile.out"
	coverageReport = "coverage.txt"
)

func (NewInstaller) Build() error {
	if (NewInstaller{}).BuildInstaller() != nil {
		return fmt.Errorf("failed to build installer")
	}
	if (NewInstaller{}).BuildConfigBuilder() != nil {
		return fmt.Errorf("failed to build config builder")
	}
	return nil
}

func (NewInstaller) BuildInstaller() error {
	output := filepath.Join(buildDir, "orch-installer")
	if err := buildInternal("./cmd", output); err != nil {
		return fmt.Errorf("failed to build config builder: %w", err)
	}
	return nil
}

func (NewInstaller) BuildConfigBuilder() error {
	output := filepath.Join(buildDir, "config-builder")
	if err := buildInternal("./cmd/config", output); err != nil {
		return fmt.Errorf("failed to build config builder: %w", err)
	}
	return nil
}

func buildInternal(srcFolder string, output string) error {
	// Ensure the build directory exists
	if err := os.MkdirAll(filepath.Join(rootDir, buildDir), 0o755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	// Build the new installer binary
	if err := sh.RunV("go", "build", "-C", rootDir, "-o", output, srcFolder); err != nil {
		return err
	}
	fmt.Printf("Installer built successfully. Run %s to start the installer.\n", filepath.Join(rootDir, output))
	return nil
}

func (NewInstaller) Test() error {
	if err := os.Chdir(rootDir); err != nil {
		return fmt.Errorf("failed to change directory to %s: %w", rootDir, err)
	}

	defer func() {
		if err := os.Chdir(".."); err != nil {
			fmt.Printf("Warning: failed to change back to root directory: %v\n", err)
		}
	}()

	// Run tests for the new installer, except for the AWS IaC tests
	// Ginkgo flags:
	// -v: verbose output
	// -r: recursive test
	// -p: parallel test
	// --skip-package: skip tests in specific packages
	// --cover: enable coverage analysis
	if err := sh.RunV("ginkgo", "-v", "-r", "-p",
		"--skip-package=targets/aws/iac",
		"--cover"); err != nil {
		return err
	}

	if _, err := os.Stat(coverProfile); err == nil {
		if err := sh.RunV("go", "tool", "cover", "-func="+coverProfile, "-o", coverageReport); err != nil {
			return fmt.Errorf("failed to generate coverage report: %w", err)
		}
		fmt.Printf("Coverage report saved to %s\n", coverageReport)
	}
	return nil
}

// Test Terraform modules
func (NewInstaller) TestAllIaC(module string) error {
	return NewInstaller{}.TestIaC("")
}

// We will not include coverage analysis for IaC tests.
func (NewInstaller) TestIaC(module string) error {
	if err := os.Chdir(rootDir); err != nil {
		return fmt.Errorf("failed to change directory to %s: %w", rootDir, err)
	}

	defer func() {
		if err := os.Chdir(".."); err != nil {
			fmt.Printf("Warning: failed to change back to root directory: %v\n", err)
		}
	}()
	if module == "" {
		if err := sh.RunV("ginkgo", "-v", "-r", "-p", "targets/aws/iac"); err != nil {
			return err
		}
	}

	modulePath := filepath.Join("targets", "aws", "iac", module)
	if _, err := os.Stat(modulePath); os.IsNotExist(err) {
		return fmt.Errorf("module %s does not exist in %s", module, modulePath)
	}

	// Run tests for the new installer, except for the AWS IaC tests
	// Ginkgo flags:
	// -v: verbose output
	// -r: recursive test
	// -p: parallel test
	if err := sh.RunV("ginkgo", "-v", "-r", "-p", modulePath); err != nil {
		return err
	}
	return nil
}

func (NewInstaller) Clean() error {
	// Remove the build directory
	dir := filepath.Join(rootDir, buildDir)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to clean build directory %s: %w", dir, err)
	}

	// Remove the cover profile file if it exists
	file := filepath.Join(rootDir, coverProfile)
	if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove %s: %w", file, err)
	}

	// Remove the cover profile file if it exists
	file = filepath.Join(rootDir, coverageReport)
	if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove %s: %w", file, err)
	}

	// Remove all *.test files
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".test" {
			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("failed to remove %s: %w", path, removeErr)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to remove *.test files: %w", err)
	}

	return nil
}

func (NewInstaller) Lint() error {
	oldWorkingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	err = os.Chdir(rootDir)
	if err != nil {
		return fmt.Errorf("failed to change directory to %s: %w", rootDir, err)
	}
	defer func() {
		err := os.Chdir(oldWorkingDir)
		if err != nil {
			fmt.Printf("Warning: failed to change back to original directory %s: %v\n", oldWorkingDir, err)
		}
	}()
	if err := sh.RunV("golangci-lint", "run", "--config", oldWorkingDir+"/.golangci.yml", "-v", "--timeout", "5m0s"); err != nil {
		return fmt.Errorf("Linter returned an error: %w", err)
	}
	return nil
}

func (NewInstaller) ValidateIaC() error {
	iacDir := filepath.Join(rootDir, "targets", "aws", "iac")

	// Find all first-level directories in the IaC directory
	dirs, err := os.ReadDir(iacDir)
	if err != nil {
		return fmt.Errorf("failed to read IaC directory %s: %w", iacDir, err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(dirs))

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		wg.Add(1)
		go func(dir os.DirEntry) {
			defer wg.Done()

			dirPath := filepath.Join(iacDir, dir.Name())

			// Run "terraform -chdir=<dirPath> init -backend=false -reconfigure -upgrade"
			if err := sh.RunV("terraform", fmt.Sprintf("-chdir=%s", dirPath), "init", "-backend=false", "-reconfigure", "-upgrade"); err != nil {
				errChan <- fmt.Errorf("failed to run terraform init in %s: %w", dirPath, err)
				return
			}

			// Run "terraform -chdir=<dirPath> validate"
			if err := sh.RunV("terraform", fmt.Sprintf("-chdir=%s", dirPath), "validate"); err != nil {
				errChan <- fmt.Errorf("failed to run terraform validate in %s: %w", dirPath, err)
				return
			}
		}(dir)
	}

	fmt.Println("Starting Terraform validation for IaC directories...")
	wg.Wait()
	close(errChan)

	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Printf("Error: %v\n", err)
		}
		return fmt.Errorf("encountered %d errors during Terraform validation", len(errors))
	}

	fmt.Println("Terraform validation completed successfully for all IaC directories.")
	return nil
}
