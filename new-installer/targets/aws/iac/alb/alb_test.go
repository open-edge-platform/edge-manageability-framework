// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_alb_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ALBTestSuite struct {
	suite.Suite
}

func TestALBTestSuite(t *testing.T) {
	suite.Run(t, new(ALBTestSuite))
}

func (s *ALBTestSuite) Test() {
}
