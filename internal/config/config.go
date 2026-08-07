package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds runtime settings loaded from environment variables.
type Config struct {
	Addr      string
	APIKey    string
	BaseURL   string
	Model     string
	DBPath    string
	DictFirst bool
	DictURL   string
}

// Load reads configuration from the environment with sensible defaults.
func Load() Config {
	cfg := Config{
		Addr:      getEnv("PHRASEMATE_ADDR", ":8080"),
		APIKey:    firstNonEmpty(os.Getenv("PHRASEMATE_API_KEY"), os.Getenv("OPENAI_API_KEY")),
		BaseURL:   normalizeBaseURL(firstNonEmpty(os.Getenv("PHRASEMATE_BASE_URL"), os.Getenv("OPENAI_BASE_URL"), "https://api.openai.com/v1")),
		Model:     firstNonEmpty(os.Getenv("PHRASEMATE_MODEL"), os.Getenv("OPENAI_MODEL"), "gpt-4o-mini"),
		DBPath:    getEnv("PHRASEMATE_DB", "data/phrasemate.db"),
		DictFirst: getEnvBool("PHRASEMATE_DICT_FIRST", true),
		DictURL:   getEnv("PHRASEMATE_DICT_URL", ""),
	}
	return cfg
}

// NormalizeBaseURL cleans and completes OpenAI-compatible base URLs.
func NormalizeBaseURL(u string) string {
	u = strings.TrimRight(strings.TrimSpace(u), "/")
	if u == "" {
		return "https://api.openai.com/v1"
	}
	if strings.HasSuffix(u, "/v1") || strings.Contains(u, "/v1/") {
		return u
	}
	// OpenAI / DeepSeek OpenAI-compatible endpoints expect /v1.
	lower := strings.ToLower(u)
	if strings.Contains(lower, "api.openai.com") || strings.Contains(lower, "api.deepseek.com") {
		return u + "/v1"
	}
	return u
}

func normalizeBaseURL(u string) string {
	return NormalizeBaseURL(u)
}

// MergePersisted overlays DB-persisted credentials onto cfg.
// Non-empty persisted values win so UI settings take effect after restart.
func (c *Config) MergePersisted(apiKey, baseURL, model string) {
	if strings.TrimSpace(apiKey) != "" {
		c.APIKey = strings.TrimSpace(apiKey)
	}
	if strings.TrimSpace(baseURL) != "" {
		c.BaseURL = NormalizeBaseURL(baseURL)
	}
	if strings.TrimSpace(model) != "" {
		c.Model = strings.TrimSpace(model)
	}
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func getEnvBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	lower := strings.ToLower(v)
	switch lower {
	case "0", "false", "no", "off":
		return false
	case "1", "true", "yes", "on":
		return true
	default:
		return fallback
	}
}

// Port returns the numeric port if Addr is like ":8080".
func (c Config) Port() int {
	addr := strings.TrimPrefix(c.Addr, ":")
	if n, err := strconv.Atoi(addr); err == nil {
		return n
	}
	return 8080
}
