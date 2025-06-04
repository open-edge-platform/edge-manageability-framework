// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package steps_aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	RequiredAvailabilityZones = 3
	PublicSubnetMaskSize      = 24
	PrivateSubnetMaskSize     = 22
	MinimumVPCCIDRMaskSize    = 20
)

type AWSUtility interface {
	GetAvailableZones(region string) ([]string, error)
	S3MoveToS3(srcRegion, srcBucket, srcKey, destRegion, destBucket, destKey string) error
}

type awsUtilityImpl struct{}

func CreateAWSUtility() AWSUtility {
	return &awsUtilityImpl{}
}

type TerraformAWSBucketBackendConfig struct {
	Region string `json:"region" yaml:"region"`
	Bucket string `json:"bucket" yaml:"bucket"`
	Key    string `json:"key" yaml:"key"`
}

func (*awsUtilityImpl) GetAvailableZones(region string) ([]string, error) {
	session, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, err
	}
	client := ec2.New(session)
	resp, err := client.DescribeAvailabilityZones(&ec2.DescribeAvailabilityZonesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("region-name"),
				Values: []*string{&region},
			},
		},
	})

	if err != nil {
		return nil, err
	}

	var zones []string
	for _, zone := range resp.AvailabilityZones {
		zones = append(zones, *zone.ZoneName)
	}

	if len(zones) < RequiredAvailabilityZones {
		return nil, fmt.Errorf("cannot get three AWS availablity zones from region %s", region)
	}
	return zones, nil
}

func FindAMIID(region string, amiName string, amiOwner string) (string, error) {
	session, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return "", err
	}
	svc := ec2.New(session)
	input := &ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("name"),
				Values: []*string{aws.String(amiName)},
			},
			{
				Name:   aws.String("owner-id"),
				Values: []*string{aws.String(amiOwner)},
			},
		},
	}
	result, err := svc.DescribeImages(input)
	if err != nil {
		return "", err
	}

	if len(result.Images) == 0 {
		return "", fmt.Errorf("no AMI found with name %s", amiName)
	}

	return *result.Images[0].ImageId, nil
}

func (*awsUtilityImpl) S3MoveToS3(srcRegion, srcBucket, srcKey, destRegion, destBucket, destKey string) error {
	session, err := session.NewSession(&aws.Config{
		Region: aws.String(srcRegion),
	})
	if err != nil {
		return err
	}

	s3Client := s3.New(session)
	copyInput := &s3.CopyObjectInput{
		Bucket:     aws.String(destBucket),
		CopySource: aws.String(fmt.Sprintf("s3://%s/%s", srcBucket, srcKey)),
		Key:        aws.String(destKey),
	}

	_, err = s3Client.CopyObject(copyInput)
	if err != nil {
		return fmt.Errorf("failed to copy object from %s/%s to %s/%s: %v", srcBucket, srcKey, destBucket, destKey, err)
	}

	return nil
}
