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
	Validation      ValidationConfig
	ShutdownTimeout time.Duration
}

type MongoDBConfig struct {
	URI      string
	Database string
}

type ValidationConfig struct {
	MaxIndividualMembers int
	MaxGroupingRules     int
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()
	v.AutomaticEnv()

	v.SetDefault("GROUP_SERVICE_ENV", string(environment.Development))
	v.SetDefault("GROUP_SERVICE_SHUTDOWN_TIMEOUT", "10s")
	v.SetDefault("GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS", 1000)
	v.SetDefault("GROUP_SERVICE_MAX_GROUPING_RULES", 10)

	cfg := Config{
		Environment: environment.Environment(v.GetString("GROUP_SERVICE_ENV")),
		HTTPAddr:    v.GetString("GROUP_SERVICE_HTTP_ADDR"),
		MongoDB: MongoDBConfig{
			URI:      v.GetString("GROUP_SERVICE_MONGODB_URI"),
			Database: v.GetString("GROUP_SERVICE_MONGODB_DATABASE"),
		},
		Validation: ValidationConfig{
			MaxIndividualMembers: v.GetInt("GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS"),
			MaxGroupingRules:     v.GetInt("GROUP_SERVICE_MAX_GROUPING_RULES"),
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
	if c.Validation.MaxIndividualMembers <= 0 {
		return fmt.Errorf("GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS must be positive")
	}
	if c.Validation.MaxGroupingRules <= 0 {
		return fmt.Errorf("GROUP_SERVICE_MAX_GROUPING_RULES must be positive")
	}
	return nil
}
