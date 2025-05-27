// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package config_test

import (
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal/config"
	"github.com/stretchr/testify/suite"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtils(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) TestSerializeAndDeserialize() {
	data := []byte(`---
version: 1
provider: "aws"
global:
  orchName: "test"
`)

	obj := config.OrchInstallerConfig{}
	err := config.DeserializeFromYAML(&obj, data)
	if !s.NoError(err, "failed to unmarshal yaml") {
		return
	}
	s.Equal(1, obj.Version)
	s.Equal("aws", obj.Provider)
	s.Equal("test", obj.Global.OrchName)
	data2, err := config.SerializeToYAML(obj)
	s.NoError(err, "failed to marshal yaml")

	obj2 := config.OrchInstallerConfig{}
	err = config.DeserializeFromYAML(&obj2, data2)
	s.NoError(err, "failed to unmarshal yaml")
	s.Equal(obj.Version, obj2.Version)
	s.Equal(obj.Provider, obj2.Provider)
	s.Equal(obj.Global.OrchName, obj2.Global.OrchName)
}
