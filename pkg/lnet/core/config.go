// core/config.go — konfiguracja LNES z env.
//
// Wszystkie wartości mają sensowne domyślne, tylko DEEPSEEK_API_KEY wymagane dla Etapu 1.

package core

import (
	"os"
)

// Config — parametry konfiguracyjne LNES.
// Wczytywane z env przez LoadConfig().
type Config struct {
	DeepSeekAPIKey string
	BindAddr       string
	MetricsPath    string
	TokenSaverLog  string
	MaxBodySize    int64
	Routes         []Route
}

// LoadConfig wczytuje konfigurację z env + domyślne.
// Jedyny wymagany env: DEEPSEEK_API_KEY (dla Etapu 1).
func LoadConfig() *Config {
	return &Config{
		DeepSeekAPIKey: os.Getenv("DEEPSEEK_API_KEY"),
		BindAddr:       getEnvDefault("LNES_BIND_ADDR", "127.0.0.1:9998"),
		MetricsPath: getEnvDefault(
			"METRICS_PATH",
			os.ExpandEnv("$HOME/.cache/goose/echo_metrics.json"),
		),
		TokenSaverLog: getEnvDefault(
			"TOKEN_SAVER_LOG",
			os.ExpandEnv("$HOME/.local/share/lazarus/token_saver.log"),
		),
		MaxBodySize: 4 * 1024 * 1024, // 4MB
		Routes: []Route{
			{Input: "http-proxy", Output: "deepseek"},
		},
	}
}

func getEnvDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
