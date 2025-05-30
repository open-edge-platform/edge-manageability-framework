// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func CreateOrDeleteS3Bucket(bucketName string, action string) error {
	session, err := session.NewSession(&aws.Config{
		Region: aws.String("us-west-2"),
	})
	if err != nil {
		return err
	}
	s3Client := s3.New(session)
	s3Input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}

	switch action {
	case "create":
		_, err = s3Client.CreateBucket(s3Input)
		if err != nil {
			if aerr, ok := err.(s3.RequestFailure); ok && aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
				// no-op, bucket already exists
			} else {
				return err
			}
		}
	case "delete":
		s3Client.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(VPCBackendBucketKey),
		})
		_, err = s3Client.DeleteBucket(&s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
		if aerr, ok := err.(s3.RequestFailure); ok && aerr.Code() == s3.ErrCodeNoSuchBucket {
			// no-op, bucket already exists
		} else {
			return err
		}
	default:
		return fmt.Errorf("unsupported action: %s", action)
	}

	return err
}
