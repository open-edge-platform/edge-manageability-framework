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

type DemoInfraNetworkStage struct {
	WorkingDir        string
	TerraformExecPath string

	variables     DemoInfraNetworkVariables
	backendConfig AWSBackendConfig
}

func (s *DemoInfraNetworkStage) Name() string {
	return "DemoInfraNetworkStage"
}

func (s *DemoInfraNetworkStage) PreStage(ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState) *internal.OrchInstallerError {
	logger := internal.Logger()
	bucketName := fmt.Sprintf("%s-%s", config.DeploymentName, config.StateStoreBucketPostfix)
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(config.Region),
	})
	if err != nil {
		return &internal.OrchInstallerError{
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
			return &internal.OrchInstallerError{
				ErrorCode:  internal.OrchInstallerErrorCodeInternal,
				ErrorStage: s.Name(),
				ErrorStep:  "PreStage",
				ErrorMsg:   fmt.Sprintf("failed to create S3 bucket: %v", err),
			}
		}
	} else {
		logger.Infof("S3 bucket %s created successfully", bucketName)
	}

	s.variables = DemoInfraNetworkVariables{
		Name: config.DeploymentName,
		Cidr: config.NetworkCIDR,
	}
	s.backendConfig = AWSBackendConfig{
		Bucket: bucketName,
		Key:    "infra-network",
		Region: config.Region,
	}
	return nil
}

func (s *DemoInfraNetworkStage) RunStage(ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState) *internal.OrchInstallerError {
	applyTerraform := steps.OrchInstallerTerraformStep{
		Action:        runtimeState.Action,
		ExecPath:      s.TerraformExecPath,
		ModulePath:    filepath.Join(s.WorkingDir, InfraNetworkModulePath),
		Variables:     s.variables,
		BackendConfig: s.backendConfig,
		LogFile:       filepath.Join(runtimeState.LogDir, "stage-infra-network-terraform.log"),
	}
	output, err := applyTerraform.Run(ctx)

	if output != nil {
		// NOTE: this can be extract to a utility function for this stage
		if vpcID, ok := output.Output["vpc_id"]; ok {
			runtimeState.VPCID = strings.Trim(vpcID, "\"")
			return nil
		} else {
			return &internal.OrchInstallerError{
				ErrorCode:  internal.OrchInstallerErrorCodeInternal,
				ErrorStage: s.Name(),
				ErrorStep:  "RunStage",
				ErrorMsg:   "vpc_id not found in Terraform output",
			}
		}
	}
	return err
}

func (s *DemoInfraNetworkStage) PostStage(ctx context.Context, config internal.OrchInstallerConfig, runtimeState *internal.OrchInstallerRuntimeState, prevStageError *internal.OrchInstallerError) *internal.OrchInstallerError {
	return prevStageError
}
