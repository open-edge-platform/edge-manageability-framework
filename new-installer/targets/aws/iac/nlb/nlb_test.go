// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_nlb_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type NLBTestSuite struct {
	suite.Suite
}

func TestNLBTestSuite(t *testing.T) {
	suite.Run(t, new(NLBTestSuite))
}

func (s *NLBTestSuite) Test() {
}
