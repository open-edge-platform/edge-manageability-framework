package aws

import "github.com/open-edge-platform/edge-manageability-framework/installer/internal"

func CreateAWSStages(rootPath string, keepGeneratedFiles bool) []internal.OrchInstallerStage {
	return []internal.OrchInstallerStage{
		&PreInfraStage{
			RootPath:           rootPath,
			KeepGeneratedFiles: keepGeneratedFiles,
		},
		&InfraStage{
			RootPath:           rootPath,
			KeepGeneratedFiles: keepGeneratedFiles,
		},
		&PreOrchStage{
			RootPath:           rootPath,
			KeepGeneratedFiles: keepGeneratedFiles,
		},
		&OrchStage{
			RootPath:           rootPath,
			KeepGeneratedFiles: keepGeneratedFiles,
		},
		&PostOrchStage{
			RootPath:           rootPath,
			KeepGeneratedFiles: keepGeneratedFiles,
		},
	}
}
