package steps

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

type CreateAWSStateBucket struct {
	bucketName string
	session    *session.Session
}

func (s *CreateAWSStateBucket) Name() string {
	return "CreateAWSStateBucket"
}

func (s *CreateAWSStateBucket) ConfigStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	if config.DeploymentName == "" || config.StateStoreBucketPostfix == "" {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInvalidArgument,
			ErrorStep: s.Name(),
			ErrorMsg:  "DeploymentName or StateStoreBucketPostfix is not set",
		}
	}
	s.bucketName = config.DeploymentName + "-" + config.StateStoreBucketPostfix
	return runtimeState, nil
}

func (s *CreateAWSStateBucket) PreSetp(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	var err error
	s.session, err = session.NewSession(&aws.Config{
		Region: aws.String(config.Region),
	})
	if err != nil {
		return runtimeState, &internal.OrchInstallerError{
			ErrorCode: internal.OrchInstallerErrorCodeInternal,
			ErrorStep: s.Name(),
			ErrorMsg:  fmt.Sprintf("failed to create AWS session: %v", err),
		}
	}
	return runtimeState, nil
}

func (s *CreateAWSStateBucket) RunStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	logger := internal.Logger()
	s3Client := s3.New(s.session)
	s3Input := &s3.CreateBucketInput{
		Bucket: aws.String(s.bucketName),
	}
	_, err := s3Client.CreateBucket(s3Input)
	if err != nil {
		if aerr, ok := err.(s3.RequestFailure); ok && aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
			logger.Debug("S3 bucket %s already exists", s.bucketName)
		} else {
			return runtimeState, &internal.OrchInstallerError{
				ErrorCode:  internal.OrchInstallerErrorCodeInternal,
				ErrorStage: s.Name(),
				ErrorStep:  "PreStage",
				ErrorMsg:   fmt.Sprintf("failed to create S3 bucket: %v", err),
			}
		}
	} else {
		logger.Debug("S3 bucket %s created successfully", s.bucketName)
	}
	return runtimeState, nil
}

func (s *CreateAWSStateBucket) PostStep(ctx context.Context, config internal.OrchInstallerConfig, runtimeState internal.OrchInstallerRuntimeState, prevStepError *internal.OrchInstallerError) (internal.OrchInstallerRuntimeState, *internal.OrchInstallerError) {
	return runtimeState, nil
}
