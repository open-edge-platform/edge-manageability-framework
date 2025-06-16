// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_eks_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type EKSTestSuite struct {
	suite.Suite
}

func TestEKSTestSuite(t *testing.T) {
	suite.Run(t, new(EKSTestSuite))
}

func (s *EKSTestSuite) Test() {
}
