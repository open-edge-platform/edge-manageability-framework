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

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
)

func UploadStateToS3(config config.OrchInstallerConfig) error {
	if config.AWS.Region == "" {
		return fmt.Errorf("AWS region is not set")
	}
	if config.Global.OrchName == "" {
		return fmt.Errorf("OrchName is not set")
	}
	if config.Generated.DeploymentID == "" {
		return fmt.Errorf("DeploymentID is not set")
	}
	session, err := session.NewSession()
	if err != nil {
		return err
	}
	s3Client := s3.New(session, &aws.Config{
		Region: aws.String(config.AWS.Region),
	})
	configYaml, err := internal.SerializeToYAML(config)
	if err != nil {
		return err
	}
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(fmt.Sprintf("%s-%s", config.Global.OrchName, config.Generated.DeploymentID)),
		Key:    aws.String("config.yaml"),
		Body:   aws.ReadSeekCloser(bytes.NewReader(configYaml)),
	})
	if err != nil {
		return err
	}
	return nil
}
