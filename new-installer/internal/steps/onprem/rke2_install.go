// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package onprem

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

const (
	rke2BaseDir     = "/var/lib/rancher/rke2"
	rke2ImagesDir   = "/var/lib/rancher/rke2/agent/images"
	rke2ConfigFile  = "/etc/rancher/rke2/rke2.yaml"
	rke2BinaryFile  = "/usr/local/bin/rke2"
	useDebInstaller = false // Set to true if using deb package installation
)

type RKE2InstallStep struct {
	RootPath               string
	KeepGeneratedFiles     bool
	OrchConfigReaderWriter config.OrchConfigReaderWriter
	StepLabels             []string
}

func CreateRKE2InstallStep(rootPath string, keepGeneratedFiles bool, orchConfigReaderWriter config.OrchConfigReaderWriter) *RKE2InstallStep {
	return &RKE2InstallStep{
		RootPath:               rootPath,
		KeepGeneratedFiles:     keepGeneratedFiles,
		OrchConfigReaderWriter: orchConfigReaderWriter,
	}
}

func (s *RKE2InstallStep) Name() string {
	return "InstallRke2Step"
}

func (s *RKE2InstallStep) Labels() []string {
	return s.StepLabels
}

func (s *RKE2InstallStep) ConfigStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}

func (s *RKE2InstallStep) PreStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if _, err := os.Stat(rke2BinaryFile); err == nil {
		// RKE2 binary exists
		if runtimeState.Action == "install" {
			fmt.Println("RKE2 is already installed, skipping installation step")
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  "RKE2 is already installed, skipping installation step",
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}
	} else if os.IsNotExist(err) {
		// RKE2 binary does not exist
		fmt.Println("RKE2 is not installed")
	} else {
		// Some other error occurred
		fmt.Printf("Error checking RKE2 installation: %v\n", err)
		return runtimeState, &internal.OrchInstallerError{
			ErrorMsg:  fmt.Sprintf("Error checking RKE2 installation: %v", err),
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
		}
	}
	return runtimeState, nil
}

func (s *RKE2InstallStep) RunStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "uninstall" {
		fmt.Println("Running RKE2 uninstallation step")
		// Stop RKE2 service
		if err := exec.Command("sudo", "/usr/local/bin/rke2-killall.sh").Run(); err != nil {
			// Upon failure, just log the error and continue
			fmt.Printf("Failed to stop RKE2 service(may not be running), continuing with uninstall...: %s\n", err)
		}

		// Remove RKE2 service
		if err := exec.Command("sudo", "/usr/local/bin/rke2-uninstall.sh").Run(); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed to disable RKE2 service: %s", err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}
	} else if runtimeState.Action == "install" {
		fmt.Printf("Running %s run-step\n", s.Name())

		if useDebInstaller {
			var dockerUsername, dockerPassword string
			var err error
			dockerUsername = config.Onprem.DockerUsername
			dockerPassword = config.Onprem.DockerToken
			currentUser, err := user.Current()
			if err != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorMsg:  fmt.Sprintf("failed to get current user: %s", err),
					ErrorCode: internal.OrchInstallerErrorCodeInternal,
				}
			}

			var kubeConfig string
			if kubeConfig, err = installRKE2Deb(installersDir, dockerUsername, dockerPassword, currentUser.Username); err != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorMsg:  fmt.Sprintf("failed to install RKE2: %s", err),
					ErrorCode: internal.OrchInstallerErrorCodeInternal,
				}
			}

			runtimeState.Onprem.KubeConfig = kubeConfig
			fmt.Println("RKE2 installation completed successfully")

		} else {

			if err := installRKE2(ctx, installersDir); err != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorMsg:  fmt.Sprintf("failed to install RKE2: %s", err),
					ErrorCode: internal.OrchInstallerErrorCodeInternal,
				}
			}
			fmt.Println("RKE2 installation completed successfully")

			if err := createRKE2ImagesDir(rke2ImagesDir); err != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorMsg:  fmt.Sprintf("failed to create RKE2 images dir %s: %s", rke2ImagesDir, err),
					ErrorCode: internal.OrchInstallerErrorCodeInternal,
				}
			}
			fmt.Println("RKE2 images directory created successfully")

			if err := copyRKE2Images(installersDir, rke2ImagesDir); err != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorMsg:  fmt.Sprintf("failed to copy RKE2 images to %s: %s", rke2ImagesDir, err),
					ErrorCode: internal.OrchInstallerErrorCodeInternal,
				}
			}

			fmt.Println("RKE2 images copied successfully")

			if err := enableRKE2Service(ctx); err != nil {
				return runtimeState, &internal.OrchInstallerError{
					ErrorMsg:  fmt.Sprintf("failed to enable RKE2 service: %s", err),
					ErrorCode: internal.OrchInstallerErrorCodeInternal,
				}
			}

			fmt.Println("RKE2 service enabled and started successfully")
		}
	} else {
		return runtimeState, &internal.OrchInstallerError{
			ErrorMsg:  "Invalid action for RKE2 step. Expected 'install' or 'uninstall'",
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
		}
	}

	return runtimeState, nil
}

func (s *RKE2InstallStep) PostStep(ctx context.Context, config config.OrchInstallerConfig, runtimeState config.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (config.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if runtimeState.Action == "install" {
		// Set up kubeconfig for the current user
		currentUser, err := user.Current()
		if err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed to get current user: %s", err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}

		// Create the .kube directory in the user's home directory
		kubeDir := fmt.Sprintf("/home/%s/.kube", currentUser.Username)
		if err := os.MkdirAll(kubeDir, 0o700); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed to create kube dir %s: %s", kubeDir, err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}

		// Copy the RKE2 config file to the user's kube directory
		if err := exec.CommandContext(ctx, "sudo", "cp", rke2ConfigFile, fmt.Sprintf("%s/config", kubeDir)).Run(); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed to copy kube config %s to %s: %s", rke2ConfigFile, kubeDir, err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}

		// Change ownership of the kube directory and its contents to the current user
		if err := exec.CommandContext(ctx, "sudo", "chown", "-R", fmt.Sprintf("%s:%s", currentUser.Username, currentUser.Username), kubeDir).Run(); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed to chown in kube dir %s: %s", kubeDir, err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}

		// Set permissions for the kube config file
		if err := exec.CommandContext(ctx, "sudo", "chmod", "600", fmt.Sprintf("%s/config", kubeDir)).Run(); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed to chmod kube config %s: %s", fmt.Sprintf("%s/config", kubeDir), err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}

		// Set the KUBECONFIG environment variable for the current user
		if err := os.Setenv("KUBECONFIG", fmt.Sprintf("/home/%s/.kube/config", currentUser.Username)); err != nil {
			return runtimeState, &internal.OrchInstallerError{
				ErrorMsg:  fmt.Sprintf("failed to set KUBECONFIG environment variable: %s", err),
				ErrorCode: internal.OrchInstallerErrorCodeInternal,
			}
		}
	}

	return runtimeState, prevStepError
}

func installRKE2(ctx context.Context, artifactDir string) error {
	fmt.Println("Installing RKE2...")

	installerFile := filepath.Join(artifactDir, rke2InstallerFile)

	if err := os.Chmod(installerFile, 0o755); err != nil {
		return fmt.Errorf("modifying install script permissions: %w", err)
	}

	cmd := exec.CommandContext(ctx, "sudo", "env",
		fmt.Sprintf("INSTALL_RKE2_ARTIFACT_PATH=%s", artifactDir),
		fmt.Sprintf("INSTALL_RKE2_METHOD=%s", "tar"),
		fmt.Sprintf("INSTALL_RKE2_VERSION=%s", rke2Version),
		"sh", "-c", installerFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run the install script: %w", err)
	}

	return nil
}

func createRKE2ImagesDir(imagesDir string) error {
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return fmt.Errorf("failed to create images directory: %w", err)
	}

	return nil
}

func copyRKE2Images(source, destination string) error {
	for _, image := range []string{
		rke2ImagesPackage,
		rke2CalicoImagePackage,
	} {
		src := fmt.Sprintf("%s/%s", source, image)
		dst := fmt.Sprintf("%s/%s", destination, image)

		input, err := os.Open(src)
		if err != nil {
			return fmt.Errorf("failed to open source image %s: %w", src, err)
		}
		defer input.Close()

		output, err := os.Create(dst)
		if err != nil {
			return fmt.Errorf("failed to create destination image %s: %w", dst, err)
		}
		defer output.Close()

		if _, err := io.Copy(output, input); err != nil {
			return fmt.Errorf("failed to copy image from %s to %s: %w", src, dst, err)
		}
	}

	return nil
}

func enableRKE2Service(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sudo", "systemctl", "enable", "--now", "rke2-server.service")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable rke2-server.service: %w", err)
	}

	cmd = exec.CommandContext(ctx, "sudo", "systemctl", "start", "rke2-server.service")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable rke2-server.service: %w", err)
	}

	cmd = exec.CommandContext(ctx, "sudo", "systemctl", "is-active", "rke2-server.service")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check rke2-server.service status: %w", err)
	}
	if string(output) != "active\n" {
		return fmt.Errorf("RKE2 server is not in active (running) state")
	}

	return nil
}

// TODO: Remove this function once the installer is fully migrated from the deb package to the tarball installation method.
func installRKE2Deb(debDirName, dockerUsername, dockerPassword, currentUser string) (string, error) {
	fmt.Println("Installing RKE2...")
	var cmd *exec.Cmd
	var kubeconfig string
	if dockerUsername != "" && dockerPassword != "" {
		fmt.Println("Docker credentials provided. Installing RKE2 with Docker credentials")
		cmd = exec.Command("sudo", "env",
			fmt.Sprintf("DOCKER_USERNAME=%s", dockerUsername),
			fmt.Sprintf("DOCKER_PASSWORD=%s", dockerPassword),
			"NEEDRESTART_MODE=a", "DEBIAN_FRONTEND=noninteractive",
			"apt-get", "install", "-y",
			fmt.Sprintf("%s/onprem-ke-installer_%s_amd64.deb", debDirName, orchVersion),
		)
	} else {
		cmd = exec.Command("sudo",
			"NEEDRESTART_MODE=a", "DEBIAN_FRONTEND=noninteractive",
			"apt-get", "install", "-y",
			fmt.Sprintf("%s/onprem-ke-installer_%s_amd64.deb", debDirName, orchVersion),
		)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to install RKE2: %w", err)
	}
	fmt.Println("OS level configuration installed and RKE2 Installed")

	kubeDir := fmt.Sprintf("/home/%s/.kube", currentUser)
	if err := os.MkdirAll(kubeDir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create kube dir: %w", err)
	}

	rke2ConfigPath := "/etc/rancher/rke2/rke2.yaml"
	if err := exec.Command("sudo", "cp", rke2ConfigPath, fmt.Sprintf("%s/config", kubeDir)).Run(); err != nil {
		return "", fmt.Errorf("failed to copy kube config: %w", err)
	}
	// Read config file from /etc/rancher/rke2/rke2.yaml and store the value as string in a variable

	rke2ConfigBytes, err := os.ReadFile(rke2ConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to read RKE2 config file %s: %w", rke2ConfigPath, err)
	}
	kubeconfig = string(rke2ConfigBytes)

	if err := exec.Command("sudo", "chown", "-R", fmt.Sprintf("%s:%s", currentUser, currentUser), kubeDir).Run(); err != nil {
		return "", fmt.Errorf("failed to chown kube dir: %w", err)
	}
	if err := exec.Command("sudo", "chmod", "600", fmt.Sprintf("%s/config", kubeDir)).Run(); err != nil {
		return "", fmt.Errorf("failed to chmod kube config: %w", err)
	}
	os.Setenv("KUBECONFIG", fmt.Sprintf("/home/%s/.kube/config", currentUser))
	return kubeconfig, nil
}
