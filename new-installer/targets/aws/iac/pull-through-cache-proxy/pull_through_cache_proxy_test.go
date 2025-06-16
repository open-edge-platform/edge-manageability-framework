// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package aws_iac_pfc_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type PFCTestSuite struct {
	suite.Suite
}

func TestPFCTestSuite(t *testing.T) {
	suite.Run(t, new(PFCTestSuite))
}

func (s *PFCTestSuite) Test() {
}
