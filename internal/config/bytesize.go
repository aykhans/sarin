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

// UnmarshalYAML reads the raw scalar so a size parses the same way here as on the command line.
func (size *byteSize) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected a size, but got %s", yamlNodeName(node))
	}

	return size.Set(node.Value)
}
