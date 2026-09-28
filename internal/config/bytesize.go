package config

import (
	"flag"
	"fmt"
	"math"
	"strconv"
	"strings"

	"go.aykhans.me/sarin/internal/types"
	"go.yaml.in/yaml/v4"
)

var (
	_ flag.Value       = (*byteSize)(nil)
	_ yaml.Unmarshaler = (*byteSize)(nil)
)

// byteSize is a size in bytes that reads and prints unit suffixes such as 10MiB.
type byteSize uint64

// byteSizeUnits run from the largest scale down, so formatting picks the largest unit
// that fits. A bare "K" is out: the common libraries disagree on whether it counts
// 1000 or 1024 bytes.
var byteSizeUnits = []struct {
	suffix string
	scale  uint64
}{
	{"TiB", 1 << 40},
	{"TB", 1000 * 1000 * 1000 * 1000},
	{"GiB", 1 << 30},
	{"GB", 1000 * 1000 * 1000},
	{"MiB", 1 << 20},
	{"MB", 1000 * 1000},
	{"KiB", 1 << 10},
	{"KB", 1000},
	{"B", 1},
}

// parseByteSize reads a byte count, with or without a unit suffix.
func parseByteSize(value string) (uint64, error) {
	text := strings.TrimSpace(value)

	// The suffix is split off by index, so no case mapping can shift the digits.
	digitsEnd := len(text)
	for digitsEnd > 0 && isASCIILetter(text[digitsEnd-1]) {
		digitsEnd--
	}

	scale := uint64(1)
	if suffix := text[digitsEnd:]; suffix != "" {
		unitScale, ok := byteSizeScale(suffix)
		if !ok {
			return 0, types.ErrByteSizeInvalid
		}
		scale = unitScale
	}

	size, err := strconv.ParseUint(strings.TrimSpace(text[:digitsEnd]), 10, 64)
	if err != nil {
		return 0, types.ErrByteSizeInvalid
	}
	if size > math.MaxUint64/scale {
		return 0, types.ErrByteSizeInvalid
	}
	return size * scale, nil
}

// byteSizeScale looks up a unit suffix, ignoring case.
func byteSizeScale(suffix string) (uint64, bool) {
	for _, unit := range byteSizeUnits {
		if strings.EqualFold(suffix, unit.suffix) {
			return unit.scale, true
		}
	}
	return 0, false
}

func isASCIILetter(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}

// formatByteSize renders a byte count so that parseByteSize reads back the same value.
func formatByteSize(size uint64) string {
	for _, unit := range byteSizeUnits {
		if unit.scale > 1 && size >= unit.scale && size%unit.scale == 0 {
			return strconv.FormatUint(size/unit.scale, 10) + unit.suffix
		}
	}
	return strconv.FormatUint(size, 10)
}

func (size byteSize) String() string {
	return formatByteSize(uint64(size))
}

// Set fills the size from a command line value.
func (size *byteSize) Set(value string) error {
	parsed, err := parseByteSize(value)
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
