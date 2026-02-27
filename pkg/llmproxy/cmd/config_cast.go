package cmd

import (
	"unsafe"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

// castToInternalConfig converts an internal config pointer to the same internal type.
func castToInternalConfig(cfg *internalconfig.Config) *internalconfig.Config {
	return cfg
}

// castToSDKConfig converts a internal/config.Config pointer to an sdk/config.Config pointer.
// This is safe because sdk/config.Config is an alias for internal/config.Config.
func castToSDKConfig(cfg *internalconfig.Config) *sdkconfig.Config {
	return (*sdkconfig.Config)(unsafe.Pointer(cfg))
}
