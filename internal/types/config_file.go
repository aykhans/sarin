package types

import (
	"path/filepath"
	"strings"
)

type ConfigFileType string

const (
	ConfigFileTypeUnknown ConfigFileType = "unknown"
	ConfigFileTypeYAML    ConfigFileType = "yaml/yml"
)

type ConfigFile struct {
	path  string
	_type ConfigFileType
}

func (configFile ConfigFile) Path() string {
	return configFile.path
}

func (configFile ConfigFile) Type() ConfigFileType {
	return configFile._type
}

func ParseConfigFile(configFileRaw string) *ConfigFile {
	// TODO: Improve file type detection
	// (e.g., use magic bytes or content inspection instead of relying solely on file extension)

	configFileParsed := &ConfigFile{
		path: configFileRaw,
	}

	configFileExtension, _ := strings.CutPrefix(filepath.Ext(configFileRaw), ".")

	switch strings.ToLower(configFileExtension) {
	case "yml", "yaml":
		configFileParsed._type = ConfigFileTypeYAML
	default:
		configFileParsed._type = ConfigFileTypeUnknown
	}

	return configFileParsed
}
