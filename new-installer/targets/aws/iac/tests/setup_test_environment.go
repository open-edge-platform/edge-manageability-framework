// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_test

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
)

func setupVPC(name string, region string) (string, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}
	ec2Client := ec2.New(sess)

	newVPC, err := ec2Client.CreateVpc(&ec2.CreateVpcInput{
		CidrBlock: aws.String("192.168.0.0/16"),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("vpc"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Environment"),
						Value: aws.String("Test"),
					},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create VPC: %v", err)
	}
	return *newVPC.Vpc.VpcId, nil
}

func setupSubnets(vpcId string, region string) (string, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}
	ec2Client := ec2.New(sess)

	subnetInput := &ec2.CreateSubnetInput{
		VpcId:     aws.String(vpcId),
		CidrBlock: aws.String("192.168.0.0/16"),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("subnet"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Environment"),
						Value: aws.String("Test"),
					},
				},
			},
		},
	}

	subnetOutput, err := ec2Client.CreateSubnet(subnetInput)
	if err != nil {
		return "", fmt.Errorf("failed to create subnet: %v", err)
	}
	subnetId := *subnetOutput.Subnet.SubnetId

	return subnetId, nil
}

func setupEKS(name string, region string, subnets []string) (string, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}
	eksClient := eks.New(sess)

	clusterInput := &eks.CreateClusterInput{
		Name:    &name,
		RoleArn: aws.String("arn:aws:iam::123456789012:role/eks-cluster-role"),
		ResourcesVpcConfig: &eks.VpcConfigRequest{
			SubnetIds:        aws.StringSlice(subnets),
			SecurityGroupIds: []*string{aws.String("sg-" + name)},
		},
	}

	clusterOutput, err := eksClient.CreateCluster(clusterInput)

	if err != nil {
		return "", fmt.Errorf("failed to create EKS cluster: %v", err)
	}
	return *clusterOutput.Cluster.Id, nil
}

func CreateTestEKSCluster(name string, region string) (string, string, string, error) {
	vpcId, err := setupVPC(name, region)
	if err != nil {
		fmt.Printf("Error setting up subnets: %v\n", err)
	}
	subnet, err := setupSubnets(vpcId, region)
	if err != nil {
		fmt.Printf("Error setting up subnets: %v\n", err)
	}
	clusterId, err := setupEKS(name, region, []string{subnet})
	if err != nil {
		fmt.Printf("Error setting up EKS cluster: %v\n", err)
	}
	return clusterId, subnet, vpcId, nil

}
