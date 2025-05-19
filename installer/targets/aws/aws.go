package aws

import "github.com/open-edge-platform/edge-manageability-framework/installer/internal"

func CreateAWSStages() []internal.OrchInstallerStage {
	return []internal.OrchInstallerStage{
		&PreInfraStage{},
	}
}
