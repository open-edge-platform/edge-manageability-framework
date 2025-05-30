// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_test

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type AWSS3BackendConfig struct {
	Region string `json:"region"`
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

func GetSubnetByID(vpcID string, subnetID string, region string) (*ec2.Subnet, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}
	svc := ec2.New(sess)
	input := &ec2.DescribeSubnetsInput{
		SubnetIds: []*string{aws.String(subnetID)},
	}

	result, err := svc.DescribeSubnets(input)
	if err != nil {
		return nil, err
	}

	if len(result.Subnets) == 0 {
		return nil, fmt.Errorf("no subnet found")
	}

	for _, subnet := range result.Subnets {
		if *subnet.VpcId == vpcID {
			return subnet, nil
		}
	}

	return nil, fmt.Errorf("no subnet found")
}

func GetInternetGatewaysByTags(region string, tags map[string][]string) ([]*ec2.InternetGateway, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}
	svc := ec2.New(sess)
	filter := make([]*ec2.Filter, 0, len(tags))
	for key, values := range tags {
		filter = append(filter, &ec2.Filter{
			Name:   aws.String("tag:" + key),
			Values: aws.StringSlice(values),
		})
	}
	input := &ec2.DescribeInternetGatewaysInput{
		Filters: filter,
	}

	result, err := svc.DescribeInternetGateways(input)
	if err != nil {
		return nil, err
	}

	if len(result.InternetGateways) == 0 {
		return nil, fmt.Errorf("no internet gateway found with the specified tags")
	}
	return result.InternetGateways, nil
}

func GetNATGatewaysByTags(region string, tags map[string][]string) ([]*ec2.NatGateway, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}
	svc := ec2.New(sess)
	filter := make([]*ec2.Filter, 0, len(tags))
	for key, values := range tags {
		filter = append(filter, &ec2.Filter{
			Name:   aws.String("tag:" + key),
			Values: aws.StringSlice(values),
		})
	}
	input := &ec2.DescribeNatGatewaysInput{
		Filter: filter,
	}

	result, err := svc.DescribeNatGateways(input)
	if err != nil {
		return nil, err
	}
	if len(result.NatGateways) == 0 {
		return nil, fmt.Errorf("no NAT gateways found with the specified tags")
	}
	return result.NatGateways, nil
}
