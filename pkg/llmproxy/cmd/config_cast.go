package cmd

import (
	"unsafe"

	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/config"
	sdkconfig "github.com/kooshapari/CLIProxyAPI/v7/sdk/config"
)

// castToInternalConfig returns the config pointer as-is.
// Both the input and output reference the same config.Config type.
func castToInternalConfig(cfg *config.Config) *config.Config {
	return cfg
}

// castToSDKConfig converts a pkg/llmproxy/config.Config pointer to an sdk/config.Config pointer.
// This is safe because sdk/config.Config is an alias for internal/config.Config, which is a subset
// of pkg/llmproxy/config.Config. The memory layout of the common fields is identical.
func castToSDKConfig(cfg *config.Config) *sdkconfig.Config {
	return (*sdkconfig.Config)(unsafe.Pointer(cfg))
}
