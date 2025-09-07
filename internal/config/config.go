package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	OpenAIKey          string `json:"-"`
	ExaKey             string `json:"-"`
	ExaEndpoint        string `json:"-"`
	ExaNumSearchResult int    `json:"-"`
}

type ConfigError struct {
	Field   string
	Value   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("configuration error for field '%s': %s", e.Field, e.Message)
}

func LoadConfig() (*Config, error) {
	config := &Config{
		OpenAIKey:          GetString("OPENAI_API_KEY", ""),
		ExaKey:             GetString("EXA_API_KEY", ""),
		ExaEndpoint:        "https://api.exa.ai/search",
		ExaNumSearchResult: 10,
	}

	return config, nil
}

func GetEnvOrDefault[T any](key string, def T, parser func(string) (T, error)) T {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	val, err := parser(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: invalid %s=%q, using default %v\n", key, raw, def)
		return def
	}
	return val
}

func StringParser(s string) (string, error) { return s, nil }
func IntParser(s string) (int, error)       { return strconv.Atoi(s) }

func GetString(key string, def string) string {
	return GetEnvOrDefault(key, def, StringParser)
}

func GetInt(key string, def int) int {
	return GetEnvOrDefault(key, def, IntParser)
}
