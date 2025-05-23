// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// Helper functions
func SerializeToYAML(config any) ([]byte, error) {
	k := koanf.New(".")
	// NOTE: Set parser to nil since we don't need to parse go struct
	err := k.Load(structs.Provider(config, "yaml"), nil)
	if err != nil {
		return nil, err
	}
	return k.Marshal(yaml.Parser())
}

func DeserializeFromYAML(config any, data []byte) error {
	v := koanf.New(".")

	err := v.Load(rawbytes.Provider(data), yaml.Parser())
	if err != nil {
		return err
	}
	return v.UnmarshalWithConf("", config, koanf.UnmarshalConf{
		Tag: "yaml",
	})
}
