package types

import (
	"math"
	"strconv"
	"strings"
)

// ByteSizeLimit is the largest size any option may ask for, 1 TiB, less where int is 32 bit.
const ByteSizeLimit = min(1<<40, math.MaxInt)

// RemoteFetchLimit bounds a file sarin downloads for itself, such as a config file or a script.
const RemoteFetchLimit = 64 << 20 // 64 MiB

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

// ParseByteSize reads a byte count, with or without a unit suffix.
// It can return the following errors:
//   - ErrByteSizeInvalid
func ParseByteSize(value string) (uint64, error) {
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
			return 0, ErrByteSizeInvalid
		}
		scale = unitScale
	}

	size, err := strconv.ParseUint(strings.TrimSpace(text[:digitsEnd]), 10, 64)
	if err != nil {
		return 0, ErrByteSizeInvalid
	}
	if size > math.MaxUint64/scale {
		return 0, ErrByteSizeInvalid
	}
	return size * scale, nil
}

// FormatByteSize renders a byte count so that ParseByteSize reads back the same value.
func FormatByteSize(size uint64) string {
	for _, unit := range byteSizeUnits {
		if unit.scale > 1 && size >= unit.scale && size%unit.scale == 0 {
			return strconv.FormatUint(size/unit.scale, 10) + unit.suffix
		}
	}
	return strconv.FormatUint(size, 10)
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
