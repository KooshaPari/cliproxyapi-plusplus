package cmd

import (
	"unsafe"

	internalconfig "github.com/kooshapari/cliproxyapi-plusplus/v6/pkg/llmproxy/config"
	sdkconfig "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/config"
)

// castToInternalConfig converts a pkg/llmproxy/config.Config pointer to the internal config type.
// The internal config maps to the same package path, so the pointer conversion is safe.
func castToInternalConfig(cfg *internalconfig.Config) *internalconfig.Config {
	return (*internalconfig.Config)(unsafe.Pointer(cfg))
}

// castToSDKConfig converts a pkg/llmproxy/config.Config pointer to an sdk/config.Config pointer.
// This is safe because sdk/config.Config is an alias for internal/config.Config, which is a subset
// of pkg/llmproxy/config.Config. The memory layout of the common fields is identical.
func castToSDKConfig(cfg *internalconfig.Config) *sdkconfig.Config {
	return (*sdkconfig.Config)(unsafe.Pointer(cfg))
}
