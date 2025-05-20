// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package internal_test

import (
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
	"github.com/stretchr/testify/suite"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtils(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) TestSerializeAndDeserialize() {
	data := []byte(`action: install
dry_run: false
log_dir: .logs
vpc_id: vpc-12345678`)

	obj := internal.OrchInstallerRuntimeState{}
	err := internal.DeserializeFromYAML(&obj, data)
	s.NoError(err, "failed to unmarshal yaml")
	s.Equal("install", obj.Action)
	s.Equal(false, obj.DryRun)
	s.Equal(".logs", obj.LogDir)
	s.Equal("vpc-12345678", obj.VPCID)
	data2, err := internal.SerializeToYAML(obj)
	s.NoError(err, "failed to marshal yaml")
	obj2 := internal.OrchInstallerRuntimeState{}
	err = internal.DeserializeFromYAML(&obj2, data2)
	s.NoError(err, "failed to unmarshal yaml")
	s.Equal(obj.Action, obj2.Action)
	s.Equal(obj.DryRun, obj2.DryRun)
	s.Equal(obj.LogDir, obj2.LogDir)
	s.Equal(obj.VPCID, obj2.VPCID)
}
