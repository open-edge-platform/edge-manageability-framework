// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
)

type AWSSM struct {
	Name string
	API  secretsmanageriface.SecretsManagerAPI
}

func NewAWSSM(name string, region string) *AWSSM {
	// if region is empty then set to us-west-2
	if region == "" {
		region = "us-west-2"
	}
	awsConfig := &aws.Config{
		Region: aws.String(region),
	}
	sess, err := session.NewSession(awsConfig)
	if err != nil {
		panic(fmt.Sprintf("not able to configure aws session: %v", err))
	}
	svc := secretsmanager.New(sess)
	return &AWSSM{
		Name: name,
		API:  svc,
	}
}

// SaveSecret saves the secret to AWS Secrets Manager
func (f *AWSSM) SaveSecret(secret string) error {
	input := &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(f.Name),
		SecretString: aws.String(secret),
	}
	_, err := f.API.PutSecretValue(input)
	if err != nil {
		return err
	}
	return nil
}

// GetSecret retrieves the secret from AWS Secrets Manager
func (f *AWSSM) GetSecret(name string) (string, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(f.Name),
	}
	result, err := f.API.GetSecretValue(input)
	if err != nil {
		return "", err
	}
	return *result.SecretString, nil
}
