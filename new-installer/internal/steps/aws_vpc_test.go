// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package steps_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type AWSVPCTestSuite struct {
	suite.Suite
}

func TestAWSVPC(t *testing.T) {
	suite.Run(t, new(AWSVPCTestSuite))
}

func (s *AWSVPCTestSuite) TestAWSVPC() {
}
