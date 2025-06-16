// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_lb_sg_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type LBSGTestSuite struct {
	suite.Suite
}

func TestLBSGTestSuite(t *testing.T) {
	suite.Run(t, new(LBSGTestSuite))
}

func (s *LBSGTestSuite) Test() {
}
