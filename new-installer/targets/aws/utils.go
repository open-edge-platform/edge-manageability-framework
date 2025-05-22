package aws

import (
	"bytes"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

func UploadStateToS3(config internal.OrchInstallerConfig) error {
	if config.Aws.Region == "" {
		return fmt.Errorf("AWS region is not set")
	}
	if config.Global.OrchName == "" {
		return fmt.Errorf("OrchName is not set")
	}
	if config.Generated.DeploymentId == "" {
		return fmt.Errorf("DeploymentId is not set")
	}
	session, err := session.NewSession()
	if err != nil {
		return err
	}
	s3Client := s3.New(session, &aws.Config{
		Region: aws.String(config.Aws.Region),
	})
	configYaml, err := internal.SerializeToYAML(config)
	if err != nil {
		return err
	}
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(fmt.Sprintf("%s-%s", config.Global.OrchName, config.Generated.DeploymentId)),
		Key:    aws.String("config.yaml"),
		Body:   aws.ReadSeekCloser(bytes.NewReader(configYaml)),
	})
	if err != nil {
		return err
	}
	return nil
}
