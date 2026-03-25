package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

const (
	DefaultPanelGitHubRepository = "https://github.com/router-for-me/Cli-Proxy-API-Management-Center"
	DefaultPprofAddr             = "127.0.0.1:8316"
)

// LoadConfig reads a YAML configuration file from the given path,
// unmarshals it into a Config struct, applies environment variable overrides,
// and returns it.
func LoadConfig(configFile string) (*Config, error) {
	return LoadConfigOptional(configFile, false)
}

// LoadConfigOptional reads YAML from configFile.
// If optional is true and the file is missing, it returns an empty Config.
func LoadConfigOptional(configFile string, optional bool) (*Config, error) {
	// Read the entire configuration file into memory.
	data, err := os.ReadFile(configFile)
	if err != nil {
		if optional {
			if os.IsNotExist(err) || errors.Is(err, syscall.EISDIR) {
				// Missing and optional: return empty config.
				return &Config{}, nil
			}
		}
		if errors.Is(err, syscall.EISDIR) {
			return nil, fmt.Errorf(
				"failed to read config file: %w (config path %q is a directory; pass a YAML file path such as /CLIProxyAPI/config.yaml)",
				err,
				configFile,
			)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// In cloud deploy mode (optional=true), if file is empty, return empty config.
	if optional && len(data) == 0 {
		return &Config{}, nil
	}

	// Unmarshal the YAML data into the Config struct.
	var cfg Config
	// Set defaults before unmarshal
	cfg.Host = ""
	cfg.LoggingToFile = false
	cfg.LogsMaxTotalSizeMB = 0
	cfg.ErrorLogsMaxFiles = 10
	cfg.UsageStatisticsEnabled = false
	cfg.DisableCooling = false
	cfg.Pprof.Enable = false
	cfg.Pprof.Addr = DefaultPprofAddr
	cfg.AmpCode.RestrictManagementToLocalhost = false
	cfg.RemoteManagement.PanelGitHubRepository = DefaultPanelGitHubRepository
	cfg.IncognitoBrowser = false

	if err = yaml.Unmarshal(data, &cfg); err != nil {
		if optional {
			// In cloud deploy mode, if YAML parsing fails, return empty config instead of error.
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Hash remote management key if plaintext is detected
	if cfg.RemoteManagement.SecretKey != "" && !looksLikeBcrypt(cfg.RemoteManagement.SecretKey) {
		hashed, errHash := hashSecret(cfg.RemoteManagement.SecretKey)
		if errHash != nil {
			return nil, fmt.Errorf("failed to hash remote management key: %w", errHash)
		}
		cfg.RemoteManagement.SecretKey = hashed

		// Persist the hashed value back to the config file
		_ = SaveConfigPreserveCommentsUpdateNestedScalar(configFile, []string{"remote-management", "secret-key"}, hashed)
	}

	cfg.RemoteManagement.PanelGitHubRepository = strings.TrimSpace(cfg.RemoteManagement.PanelGitHubRepository)
	if cfg.RemoteManagement.PanelGitHubRepository == "" {
		cfg.RemoteManagement.PanelGitHubRepository = DefaultPanelGitHubRepository
	}

	cfg.Pprof.Addr = strings.TrimSpace(cfg.Pprof.Addr)
	if cfg.Pprof.Addr == "" {
		cfg.Pprof.Addr = DefaultPprofAddr
	}

	if cfg.LogsMaxTotalSizeMB < 0 {
		cfg.LogsMaxTotalSizeMB = 0
	}

	if cfg.ErrorLogsMaxFiles < 0 {
		cfg.ErrorLogsMaxFiles = 10
	}

	// Sanitize configurations
	cfg.SanitizeGeminiKeys()
	cfg.SanitizeVertexCompatKeys()
	cfg.SanitizeCodexKeys()
	cfg.SanitizeClaudeKeys()
	cfg.SanitizeKiroKeys()
	cfg.SanitizeCursorKeys()
	cfg.SanitizeGeneratedProviders()
	cfg.SanitizeOpenAICompatibility()

	// Inject premade providers from environment
	cfg.InjectPremadeFromEnv()

	// Normalize OAuth settings
	cfg.OAuthExcludedModels = NormalizeOAuthExcludedModels(cfg.OAuthExcludedModels)
	cfg.SanitizeOAuthModelAlias()
	cfg.SanitizeOAuthUpstream()

	// Validate payload rules
	cfg.SanitizePayloadRules()

	// Apply environment variable overrides
	cfg.ApplyEnvOverrides()

	return &cfg, nil
}

// InjectPremadeFromEnv injects premade providers if their environment variables are set.
func (cfg *Config) InjectPremadeFromEnv() {
	for _, spec := range GetPremadeProviders() {
		cfg.injectPremadeFromSpec(spec.Name, spec)
	}
}

func (cfg *Config) injectPremadeFromSpec(name string, spec ProviderSpec) {
	// Check if already in config
	for _, compat := range cfg.OpenAICompatibility {
		if strings.ToLower(compat.Name) == name {
			return
		}
	}

	// Check env vars
	var apiKey string
	for _, ev := range spec.EnvVars {
		if val := os.Getenv(ev); val != "" {
			apiKey = val
			break
		}
	}
	if apiKey == "" {
		return
	}

	// Inject virtual entry
	entry := OpenAICompatibility{
		Name:    name,
		BaseURL: spec.BaseURL,
		APIKeyEntries: []OpenAICompatibilityAPIKey{
			{APIKey: apiKey},
		},
		Models: spec.DefaultModels,
	}
	cfg.OpenAICompatibility = append(cfg.OpenAICompatibility, entry)
}

// looksLikeBcrypt returns true if the provided string appears to be a bcrypt hash.
func looksLikeBcrypt(s string) bool {
	return len(s) > 4 && (s[:4] == "$2a$" || s[:4] == "$2b$" || s[:4] == "$2y$")
}

// hashSecret hashes the given secret using bcrypt.
func hashSecret(secret string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

// ApplyEnvOverrides applies environment variable overrides to the configuration.
func (cfg *Config) ApplyEnvOverrides() {
	if cfg == nil {
		return
	}

	// CLIPROXY_HOST
	if val := os.Getenv("CLIPROXY_HOST"); val != "" {
		cfg.Host = val
		log.WithField("host", val).Info("Applied CLIPROXY_HOST override")
	}

	// CLIPROXY_PORT
	if val := os.Getenv("CLIPROXY_PORT"); val != "" {
		if port, err := parseIntEnvVar(val); err == nil && port > 0 && port <= 65535 {
			cfg.Port = port
			log.WithField("port", port).Info("Applied CLIPROXY_PORT override")
		} else {
			log.WithField("value", val).Warn("Invalid CLIPROXY_PORT value, ignoring")
		}
	}

	// CLIPROXY_SECRET_KEY
	if val := os.Getenv("CLIPROXY_SECRET_KEY"); val != "" {
		if !looksLikeBcrypt(val) {
			hashed, err := hashSecret(val)
			if err != nil {
				log.WithError(err).Warn("Failed to hash CLIPROXY_SECRET_KEY, using as-is")
				cfg.RemoteManagement.SecretKey = val
			} else {
				cfg.RemoteManagement.SecretKey = hashed
			}
		} else {
			cfg.RemoteManagement.SecretKey = val
		}
		log.Info("Applied CLIPROXY_SECRET_KEY override")
	}

	// CLIPROXY_ALLOW_REMOTE
	if val := os.Getenv("CLIPROXY_ALLOW_REMOTE"); val != "" {
		if parsed, err := parseBoolEnvVar(val); err == nil {
			cfg.RemoteManagement.AllowRemote = parsed
			log.WithField("allow-remote", parsed).Info("Applied CLIPROXY_ALLOW_REMOTE override")
		} else {
			log.WithField("value", val).Warn("Invalid CLIPROXY_ALLOW_REMOTE value, ignoring")
		}
	}

	// CLIPROXY_DEBUG
	if val := os.Getenv("CLIPROXY_DEBUG"); val != "" {
		if parsed, err := parseBoolEnvVar(val); err == nil {
			cfg.Debug = parsed
			log.WithField("debug", parsed).Info("Applied CLIPROXY_DEBUG override")
		} else {
			log.WithField("value", val).Warn("Invalid CLIPROXY_DEBUG value, ignoring")
		}
	}

	// CLIPROXY_ROUTING_STRATEGY
	if val := os.Getenv("CLIPROXY_ROUTING_STRATEGY"); val != "" {
		normalized := strings.ToLower(strings.TrimSpace(val))
		switch normalized {
		case "round-robin", "roundrobin", "rr":
			cfg.Routing.Strategy = "round-robin"
			log.Info("Applied CLIPROXY_ROUTING_STRATEGY override: round-robin")
		case "fill-first", "fillfirst", "ff":
			cfg.Routing.Strategy = "fill-first"
			log.Info("Applied CLIPROXY_ROUTING_STRATEGY override: fill-first")
		default:
			log.WithField("value", val).Warn("Invalid CLIPROXY_ROUTING_STRATEGY value, ignoring")
		}
	}

	// CLIPROXY_API_KEYS
	if val := os.Getenv("CLIPROXY_API_KEYS"); val != "" {
		keys := strings.Split(val, ",")
		cfg.APIKeys = make([]string, 0, len(keys))
		for _, key := range keys {
			trimmed := strings.TrimSpace(key)
			if trimmed != "" {
				cfg.APIKeys = append(cfg.APIKeys, trimmed)
			}
		}
		if len(cfg.APIKeys) > 0 {
			log.WithField("count", len(cfg.APIKeys)).Info("Applied CLIPROXY_API_KEYS override")
		}
	}
}

// parseIntEnvVar parses an integer from an environment variable string.
func parseIntEnvVar(val string) (int, error) {
	val = strings.TrimSpace(val)
	var result int
	_, err := fmt.Sscanf(val, "%d", &result)
	return result, err
}

// parseBoolEnvVar parses a boolean from an environment variable string.
func parseBoolEnvVar(val string) (bool, error) {
	val = strings.ToLower(strings.TrimSpace(val))
	switch val {
	case "true", "yes", "1", "on":
		return true, nil
	case "false", "no", "0", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", val)
	}
}
