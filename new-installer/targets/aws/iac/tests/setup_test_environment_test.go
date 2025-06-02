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
	"github.com/aws/aws-sdk-go/service/iam"
)

func setupIAMRole(name string, region string) (string, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}
	iamClient := iam.New(sess)

	roleInput := &iam.CreateRoleInput{
		RoleName: aws.String(name + "-eks-role"),
		AssumeRolePolicyDocument: aws.String(`{
			"Version": "2012-10-17",
			"Statement": [
				{
				"Effect": "Allow",
				"Principal": {
					"Service": "eks.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
				}
			]
			}`),
	}
	if err != nil {
		return "", fmt.Errorf("failed to create IAM role: %v", err)
	}
	roleOutput, err := iamClient.CreateRole(roleInput)
	if err != nil {
		return "", fmt.Errorf("failed to create IAM role: %v", err)
	}
	return *roleOutput.Role.Arn, nil
}

func deleteIAMRole(roleName string, region string) error {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	iamClient := iam.New(sess)

	_, err = iamClient.DeleteRole(&iam.DeleteRoleInput{
		RoleName: aws.String(roleName + "-eks-role"),
	})
	if err != nil {
		return fmt.Errorf("failed to delete IAM role: %v", err)
	}
	fmt.Printf("IAM role %s deleted successfully\n", roleName)
	return nil
}

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
						Key:   aws.String("Name"),
						Value: aws.String(name + "-vpc"),
					},
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

func deleteVPC(vpcId string, region string) error {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	ec2Client := ec2.New(sess)

	_, err = ec2Client.DeleteVpc(&ec2.DeleteVpcInput{
		VpcId: aws.String(vpcId),
	})
	if err != nil {
		return fmt.Errorf("failed to delete VPC: %v", err)
	}
	fmt.Printf("VPC %s deleted successfully\n", vpcId)
	return nil
}

func setupSubnets(vpcId string, region string) ([]string, error) {
	subnets := []string{}
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return subnets, fmt.Errorf("failed to create session: %v", err)
	}
	ec2Client := ec2.New(sess)

	subnet1, err := createSubnet("vpcID-subnet-1", "192.168.0.0/18", ec2Client, vpcId, region, "usw2-az1")
	if err != nil {
		return subnets, fmt.Errorf("failed to create subnet 1: %v", err)
	}
	subnets = append(subnets, subnet1)

	subnet2, err := createSubnet("vpcID-subnet-2", "192.168.64.0/18", ec2Client, vpcId, region, "usw2-az2")
	if err != nil {
		return subnets, fmt.Errorf("failed to create subnet 2: %v", err)
	}
	subnets = append(subnets, subnet2)

	return subnets, nil
}

func createSubnet(name string, cidr string, ec2Client *ec2.EC2, vpcId string, region string, az string) (string, error) {
	subnetInput := &ec2.CreateSubnetInput{
		VpcId:              aws.String(vpcId),
		CidrBlock:          aws.String(cidr),
		AvailabilityZoneId: aws.String(az),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("subnet"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(name),
					},
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
	return *subnetOutput.Subnet.SubnetId, nil
}

func deleteSubnets(subnetIds []string, region string) error {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	ec2Client := ec2.New(sess)

	for _, subnetId := range subnetIds {
		_, err = ec2Client.DeleteSubnet(&ec2.DeleteSubnetInput{
			SubnetId: aws.String(subnetId),
		})
		if err != nil {
			return fmt.Errorf("failed to delete subnet: %v", err)
		}
		fmt.Printf("Subnet %s deleted successfully\n", subnetId)
	}
	return nil
}

func setupEKS(name string, region string, subnets []string, roleARN string) (string, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}
	eksClient := eks.New(sess)

	clusterInput := &eks.CreateClusterInput{
		Name:    &name,
		RoleArn: aws.String(roleARN),
		ResourcesVpcConfig: &eks.VpcConfigRequest{
			SubnetIds: aws.StringSlice(subnets),
		},
	}

	clusterOutput, err := eksClient.CreateCluster(clusterInput)

	if err != nil {
		return "", fmt.Errorf("failed to create EKS cluster: %v", err)
	}
	return *clusterOutput.Cluster.Name, nil
}

func deleteEKS(clusterName string, region string) error {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	eksClient := eks.New(sess)

	_, err = eksClient.DeleteCluster(&eks.DeleteClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete EKS cluster: %v", err)
	}
	fmt.Printf("EKS cluster %s deleted successfully\n", clusterName)
	return nil
}

func CreateTestEKSCluster(name string, region string) (string, []string, string, error) {
	roleARN, err := setupIAMRole(name, region)
	if err != nil {
		fmt.Printf("Error setting up IAM role: %v\n", err)
		return "", []string{}, "", err
	}

	vpcId, err := setupVPC(name, region)
	if err != nil {
		fmt.Printf("Error setting up subnets: %v\n", err)
		deleteIAMRole(name, region)
		return "", []string{}, "", err
	}

	subnets, err := setupSubnets(vpcId, region)
	if err != nil {
		fmt.Printf("Error setting up subnets: %v\n", err)
		deleteVPC(vpcId, region)
		deleteIAMRole(name, region)
		return "", []string{}, vpcId, err
	}
	clusterName, err := setupEKS(name, region, subnets, roleARN)
	if err != nil {
		fmt.Printf("Error setting up EKS cluster: %v\n", err)
		deleteSubnets(subnets, region)
		deleteVPC(vpcId, region)
		deleteIAMRole(name, region)
		return "", subnets, vpcId, err
	}
	return clusterName, subnets, vpcId, nil
}

func DeleteTestEKSCluster(name string, subnets []string, vpcId string, region string) error {
	err := deleteEKS(name, region)
	if err != nil {
		return fmt.Errorf("failed to delete EKS cluster: %v", err)
	}

	err = deleteSubnets(subnets, region)
	if err != nil {
		return fmt.Errorf("failed to delete subnets: %v", err)
	}

	err = deleteVPC(vpcId, region)
	if err != nil {
		return fmt.Errorf("failed to delete VPC: %v", err)
	}

	err = deleteIAMRole(name, region)
	if err != nil {
		return fmt.Errorf("failed to delete IAM role: %v", err)
	}

	fmt.Println("Successfully deleted EKS cluster and associated resources")

	return nil
}
