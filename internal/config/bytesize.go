package config

import (
	"flag"
	"fmt"

	"go.aykhans.me/sarin/internal/types"
	"go.yaml.in/yaml/v4"
)

var (
	_ flag.Value       = (*byteSize)(nil)
	_ yaml.Unmarshaler = (*byteSize)(nil)
)

// byteSize is a size in bytes that reads and prints unit suffixes such as 10MiB.
type byteSize uint64

func (size byteSize) String() string {
	return types.FormatByteSize(uint64(size))
}

// Set fills the size from a command line value.
func (size *byteSize) Set(value string) error {
	parsed, err := types.ParseByteSize(value)
	if err != nil {
		return err
	}
	*size = byteSize(parsed)
	return nil
}

// UnmarshalYAML accepts both a YAML integer and a string with a unit.
func (size *byteSize) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected a size, but got %s", node.Tag)
	}

	if node.Tag == "!!int" {
		var parsed uint64
		if err := node.Decode(&parsed); err != nil {
			return types.ErrByteSizeInvalid
		}
		*size = byteSize(parsed)
		return nil
	}

	return size.Set(node.Value)
}
