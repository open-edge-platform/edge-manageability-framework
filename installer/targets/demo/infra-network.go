package demo

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/steps"
)

const (
	InfraNetworkModulePath = "installer/targets/demo/iac/infra-network"
)

type DemoInfraNetworkVariables struct {
	Name string `yaml:"name"`
	Cidr string `yaml:"cidr"`
}

type AWSBackendConfig struct {
	Bucket string `yaml:"bucket"`
	Key    string `yaml:"key"`
	Region string `yaml:"region"`
}

type DemoInfraNetworkStageRuntimeState struct {
	// Passing between prestage and stage
	Variables     DemoInfraNetworkVariables `yaml:"variables"`
	BackendConfig AWSBackendConfig          `yaml:"backend_config"`

	// Passing to infra stage
	VPCID string `yaml:"vpc_id" validate:"required"`

	// Inhert from installer input
	Action string `yaml:"action" validate:"required,oneof=install upgrade uninstall"`
	LogDir string `yaml:"log_path"`
}

type DemoInfraNetworkStage struct {
	WorkingDir        string
	TerraformExecPath string
}

func (s *DemoInfraNetworkStage) Name() string {
	return "DemoInfraNetworkStage"
}

func (s *DemoInfraNetworkStage) PreStage(ctx *context.Context, installerInput internal.OrchInstallerInput, prevStageOutput internal.RuntimeState) (internal.RuntimeState, *internal.OrchInstallerError) {
	installerOutput, ok := prevStageOutput.(*internal.OrchInstallerRuntimeState)
	if !ok {
		return nil, &internal.OrchInstallerError{
			ErrorCode:  internal.OrchInstallerErrorCodeInternal,
			ErrorStage: s.Name(),
			ErrorStep:  "PreStage",
			ErrorMsg:   "failed to cast previous stage output to OrchInstallerOutput",
		}
	}

	logger := internal.Logger()
	bucketName := fmt.Sprintf("%s-%s", installerInput.DeploymentName, installerInput.StateStoreBucketPostfix)
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(installerInput.Region),
	})
	if err != nil {
		return nil, &internal.OrchInstallerError{
			ErrorCode:  internal.OrchInstallerErrorCodeInternal,
			ErrorStage: s.Name(),
			ErrorStep:  "PreStage",
			ErrorMsg:   fmt.Sprintf("failed to create AWS session: %v", err),
		}
	}
	s3Client := s3.New(sess)
	s3Input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}
	_, err = s3Client.CreateBucket(s3Input)
	if err != nil {
		if aerr, ok := err.(s3.RequestFailure); ok && aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
			logger.Infof("S3 bucket %s already exists", bucketName)
		} else {
			return nil, &internal.OrchInstallerError{
				ErrorCode:  internal.OrchInstallerErrorCodeInternal,
				ErrorStage: s.Name(),
				ErrorStep:  "PreStage",
				ErrorMsg:   fmt.Sprintf("failed to create S3 bucket: %v", err),
			}
		}
	} else {
		logger.Infof("S3 bucket %s created successfully", bucketName)
	}

	variables := DemoInfraNetworkVariables{
		Name: installerInput.DeploymentName,
		Cidr: installerInput.NetworkCIDR,
	}
	backendConfig := AWSBackendConfig{
		Bucket: bucketName,
		Key:    "infra-network",
		Region: installerInput.Region,
	}

	return &DemoInfraNetworkStageRuntimeState{
		Action:        installerOutput.Action,
		Variables:     variables,
		BackendConfig: backendConfig,
		LogDir:        installerOutput.LogDir,
	}, nil
}

func (s *DemoInfraNetworkStage) RunStage(ctx *context.Context, installerInput internal.OrchInstallerInput, prevStageOutput internal.RuntimeState) (internal.RuntimeState, *internal.OrchInstallerError) {
	prevOutput, ok := prevStageOutput.(*DemoInfraNetworkStageRuntimeState)
	if !ok {
		return nil, &internal.OrchInstallerError{
			ErrorCode:  internal.OrchInstallerErrorCodeInternal,
			ErrorStage: s.Name(),
			ErrorStep:  "RunStage",
			ErrorMsg:   "failed to cast previous stage output to DemoInfraNetworkStageRuntimeState",
		}
	}

	applyTerraform := steps.OrchInstallerTerraformStep{
		Action:        prevOutput.Action,
		ExecPath:      s.TerraformExecPath,
		ModulePath:    filepath.Join(s.WorkingDir, InfraNetworkModulePath),
		Variables:     prevOutput.Variables,
		BackendConfig: prevOutput.BackendConfig,
		LogFile:       filepath.Join(prevOutput.LogDir, "stage-infra-network-terraform.log"),
	}
	output, err := applyTerraform.Run(ctx)

	rs := &DemoInfraNetworkStageRuntimeState{
		Action: prevOutput.Action,
		LogDir: prevOutput.LogDir,
	}

	if output != nil {
		// NOTE: this can be extract to a utility function for this stage
		if vpcID, ok := output.Output["vpc_id"]; ok {
			rs.VPCID = strings.Trim(vpcID, "\"")
		} else {
			return nil, &internal.OrchInstallerError{
				ErrorCode:  internal.OrchInstallerErrorCodeInternal,
				ErrorStage: s.Name(),
				ErrorStep:  "RunStage",
				ErrorMsg:   "vpc_id not found in Terraform output",
			}
		}
	}
	return rs, err
}

func (s *DemoInfraNetworkStage) PostStage(ctx *context.Context, installerInput internal.OrchInstallerInput, prevStageOutput internal.RuntimeState, prevStageError *internal.OrchInstallerError) (internal.RuntimeState, *internal.OrchInstallerError) {
	return prevStageOutput, prevStageError
}
