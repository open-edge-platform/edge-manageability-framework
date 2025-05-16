package internal

import (
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// Helper functions
func SerializeToYAML(runtimeState any) ([]byte, error) {
	k := koanf.New(".")
	// NOTE: Set parser to nil since we don't need to parse go struct
	err := k.Load(structs.Provider(runtimeState, "yaml"), nil)
	if err != nil {
		return nil, err
	}
	return k.Marshal(yaml.Parser())
}

func DeserializeFromYAML(runtimeState any, data []byte) error {
	v := koanf.New(".")

	err := v.Load(rawbytes.Provider(data), yaml.Parser())
	if err != nil {
		return err
	}
	return v.UnmarshalWithConf("", runtimeState, koanf.UnmarshalConf{
		Tag: "yaml",
	})
}
