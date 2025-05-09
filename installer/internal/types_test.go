package internal_test

import (
	"testing"

	"github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

type TestStruct struct {
	Foo string `yaml:"foo"`
	Baz string `yaml:"baz"`
}

func TestSerializeAndDeserialize(t *testing.T) {
	data := []byte(`---
foo: "bar"
baz: "baz"`)

	obj := &TestStruct{}
	err := internal.DeserializeFromYAML(obj, data)
	if err != nil {
		t.Fatalf("failed to unmarshal yaml: %v", err)
	}

	if obj.Foo != "bar" {
		t.Fatalf("expected Foo to be 'bar', got '%s'", obj.Foo)
	}
	if obj.Baz != "baz" {
		t.Fatalf("expected Baz to be 'baz', got '%s'", obj.Baz)
	}

	data2, err := internal.SerializeToYAML(obj)
	if err != nil {
		t.Fatalf("failed to marshal yaml: %v", err)
	}
	// NOTE: The serialized data may not match the original input due to formatting or key order differences.
	obj2 := &TestStruct{}
	err = internal.DeserializeFromYAML(obj2, data2)
	if err != nil {
		t.Fatalf("failed to unmarshal yaml: %v", err)
	}
	if obj2.Foo != "bar" {
		t.Fatalf("expected Foo to be 'bar', got '%s'", obj2.Foo)
	}
	if obj2.Baz != "baz" {
		t.Fatalf("expected Baz to be 'baz', got '%s'", obj2.Baz)
	}
}
