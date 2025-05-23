// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"bytes"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

func UploadStateToS3(orchConfig config.OrchInstallerConfig) error {
	if orchConfig.AWS.Region == "" {
		return fmt.Errorf("AWS region is not set")
	}
	if orchConfig.Global.OrchName == "" {
		return fmt.Errorf("OrchName is not set")
	}
	if orchConfig.Generated.DeploymentID == "" {
		return fmt.Errorf("DeploymentID is not set")
	}
	session, err := session.NewSession()
	if err != nil {
		return err
	}
	s3Client := s3.New(session, &aws.Config{
		Region: aws.String(orchConfig.AWS.Region),
	})
	configYaml, err := config.SerializeToYAML(orchConfig)
	if err != nil {
		return err
	}
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(fmt.Sprintf("%s-%s", orchConfig.Global.OrchName, orchConfig.Generated.DeploymentID)),
		Key:    aws.String("config.yaml"),
		Body:   aws.ReadSeekCloser(bytes.NewReader(configYaml)),
	})
	if err != nil {
		return err
	}
	return nil
}
