package demo

import (
	"context"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

const (
	terraformVersion = "1.9.5"
)

func CreateDemoStages(workingDir string) []internal.OrchInstallerStage {
	logger := internal.Logger()
	installer := &releases.ExactVersion{
		Product: product.Terraform,
		Version: version.Must(version.NewVersion(terraformVersion)),
	}

	execPath, err := installer.Install(context.Background())
	if err != nil {
		logger.Fatalf("error installing Terraform: %s", err)
	}

	return []internal.OrchInstallerStage{
		&DemoInfraNetworkStage{
			WorkingDir:        workingDir,
			TerraformExecPath: execPath,
		},
		&DemoInfraStage{
			WorkingDir:        workingDir,
			TerraformExecPath: execPath,
		},
	}
}
