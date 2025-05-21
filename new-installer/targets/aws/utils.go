package aws

import (
	"bytes"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

func UploadRuntimeStateToS3(bucketName string, region string, runtimeState internal.OrchInstallerRuntimeState) error {
	session, err := session.NewSession()
	if err != nil {
		return err
	}
	s3Client := s3.New(session, &aws.Config{
		Region: aws.String(region),
	})
	runtimeStateYaml, err := internal.SerializeToYAML(runtimeState)
	if err != nil {
		return err
	}
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String("runtime-state.yaml"),
		Body:   aws.ReadSeekCloser(bytes.NewReader(runtimeStateYaml)),
	})
	if err != nil {
		return err
	}
	return nil
}
