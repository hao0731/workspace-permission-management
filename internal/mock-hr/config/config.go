package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

type Config struct {
	Environment     environment.Environment
	HTTPAddr        string
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()
	v.AutomaticEnv()

	v.SetDefault("MOCK_HR_ENV", string(environment.Development))
	v.SetDefault("MOCK_HR_SHUTDOWN_TIMEOUT", "10s")

	cfg := Config{
		Environment:     environment.Environment(v.GetString("MOCK_HR_ENV")),
		HTTPAddr:        v.GetString("MOCK_HR_HTTP_ADDR"),
		ShutdownTimeout: v.GetDuration("MOCK_HR_SHUTDOWN_TIMEOUT"),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if !environment.IsValidEnvironment(c.Environment) {
		return fmt.Errorf("%w: MOCK_HR_ENV must be %q or %q", environment.ErrInvalidEnv, environment.Development, environment.Production)
	}
	if strings.TrimSpace(c.HTTPAddr) == "" {
		return fmt.Errorf("MOCK_HR_HTTP_ADDR is required")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("MOCK_HR_SHUTDOWN_TIMEOUT must be positive")
	}
	return nil
}
