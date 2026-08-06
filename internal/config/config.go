package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds runtime settings loaded from environment variables.
type Config struct {
	Addr    string
	APIKey  string
	BaseURL string
	Model   string
	DBPath  string
}

// Load reads configuration from the environment with sensible defaults.
func Load() Config {
	cfg := Config{
		Addr:    getEnv("PHRASEMATE_ADDR", ":8080"),
		APIKey:  firstNonEmpty(os.Getenv("PHRASEMATE_API_KEY"), os.Getenv("OPENAI_API_KEY")),
		BaseURL: normalizeBaseURL(firstNonEmpty(os.Getenv("PHRASEMATE_BASE_URL"), os.Getenv("OPENAI_BASE_URL"), "https://api.openai.com/v1")),
		Model:   firstNonEmpty(os.Getenv("PHRASEMATE_MODEL"), os.Getenv("OPENAI_MODEL"), "gpt-4o-mini"),
		DBPath:  getEnv("PHRASEMATE_DB", "data/phrasemate.db"),
	}
	return cfg
}

func normalizeBaseURL(u string) string {
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

// Port returns the numeric port if Addr is like ":8080".
func (c Config) Port() int {
	addr := strings.TrimPrefix(c.Addr, ":")
	if n, err := strconv.Atoi(addr); err == nil {
		return n
	}
	return 8080
}
