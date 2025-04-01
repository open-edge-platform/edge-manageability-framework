// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/open-edge-platform/edge-manageability-framework/internal/secrets"
	"github.com/stretchr/testify/mock"
)

type mockSMClient struct {
	secretsmanageriface.SecretsManagerAPI
	mock.Mock
}

func (m *mockSMClient) GetSecretValue(_ *secretsmanager.GetSecretValueInput,
) (*secretsmanager.GetSecretValueOutput, error) {
	args := m.Called()
	return args.Get(0).(*secretsmanager.GetSecretValueOutput), args.Error(1)
}

func (m *mockSMClient) PutSecretValue(_ *secretsmanager.PutSecretValueInput,
) (*secretsmanager.PutSecretValueOutput, error) {
	args := m.Called()
	return args.Get(0).(*secretsmanager.PutSecretValueOutput), args.Error(1)
}

var _ = Describe("AWS Secrets Manager", func() {
	var client *mockSMClient

	BeforeEach(func() {
		client = &mockSMClient{}
	})

	Context("Secrets manager", func() {
		It("should return the secret", func() {
			client.On("GetSecretValue").Return(
				&secretsmanager.GetSecretValueOutput{
					SecretString: aws.String("mockSecret"),
				}, nil)

			awssm := &secrets.AWSSM{
				Name: "test",
				API:  client,
			}
			result, err := awssm.GetSecret("")
			Expect(result).To(Equal("mockSecret"))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should save the secret", func() {
			client.On("PutSecretValue").Return(
				&secretsmanager.PutSecretValueOutput{}, nil)

			awssm := &secrets.AWSSM{
				Name: "test",
				API:  client,
			}
			err := awssm.SaveSecret("mockSecret")
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error if update fails", func() {
			client.On("PutSecretValue").Return(
				&secretsmanager.PutSecretValueOutput{}, fmt.Errorf("some error"))

			awssm := &secrets.AWSSM{
				Name: "test",
				API:  client,
			}
			err := awssm.SaveSecret("mockSecret")
			Expect(err).To(MatchError("some error"))
		})
	})
})
