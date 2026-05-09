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
	MongoDB         MongoDBConfig
	ShutdownTimeout time.Duration
}

type MongoDBConfig struct {
	URI      string
	Database string
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()
	v.AutomaticEnv()

	v.SetDefault("GROUP_SERVICE_ENV", string(environment.Development))
	v.SetDefault("GROUP_SERVICE_SHUTDOWN_TIMEOUT", "10s")

	cfg := Config{
		Environment: environment.Environment(v.GetString("GROUP_SERVICE_ENV")),
		HTTPAddr:    v.GetString("GROUP_SERVICE_HTTP_ADDR"),
		MongoDB: MongoDBConfig{
			URI:      v.GetString("GROUP_SERVICE_MONGODB_URI"),
			Database: v.GetString("GROUP_SERVICE_MONGODB_DATABASE"),
		},
		ShutdownTimeout: v.GetDuration("GROUP_SERVICE_SHUTDOWN_TIMEOUT"),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if !environment.IsValidEnvironment(c.Environment) {
		return fmt.Errorf("%w: GROUP_SERVICE_ENV must be %q or %q", environment.ErrInvalidEnv, environment.Development, environment.Production)
	}
	required := map[string]string{
		"GROUP_SERVICE_HTTP_ADDR":        c.HTTPAddr,
		"GROUP_SERVICE_MONGODB_URI":      c.MongoDB.URI,
		"GROUP_SERVICE_MONGODB_DATABASE": c.MongoDB.Database,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("GROUP_SERVICE_SHUTDOWN_TIMEOUT must be positive")
	}
	return nil
}
