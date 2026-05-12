package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

type Config struct {
	Environment           environment.Environment
	HTTPAddr              string
	MongoDB               MongoDBConfig
	NATS                  NATSConfig
	HR                    HRConfig
	ResourceMappings      ResourceMappings
	CommandPublishTimeout time.Duration
	ShutdownTimeout       time.Duration
}

type MongoDBConfig struct {
	URI      string
	Database string
}

type NATSConfig struct {
	URL string
}

type HRConfig struct {
	BaseURL string
}

type ResourceMappings struct {
	Documents ResourceMapping
	Tasks     ResourceMapping
	Drive     ResourceMapping
}

type ResourceMapping struct {
	AppName      string
	ResourceType string
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()
	v.AutomaticEnv()

	v.SetDefault("WORKSPACE_SERVICE_ENV", string(environment.Development))
	v.SetDefault("WORKSPACE_SERVICE_COMMAND_PUBLISH_TIMEOUT", "15s")
	v.SetDefault("WORKSPACE_SERVICE_SHUTDOWN_TIMEOUT", "10s")

	cfg := Config{
		Environment: environment.Environment(v.GetString("WORKSPACE_SERVICE_ENV")),
		HTTPAddr:    v.GetString("WORKSPACE_SERVICE_HTTP_ADDR"),
		MongoDB: MongoDBConfig{
			URI:      v.GetString("WORKSPACE_SERVICE_MONGODB_URI"),
			Database: v.GetString("WORKSPACE_SERVICE_MONGODB_DATABASE"),
		},
		NATS: NATSConfig{URL: v.GetString("WORKSPACE_SERVICE_NATS_URL")},
		HR:   HRConfig{BaseURL: v.GetString("WORKSPACE_SERVICE_HR_BASE_URL")},
		ResourceMappings: ResourceMappings{
			Documents: ResourceMapping{
				AppName:      v.GetString("WORKSPACE_SERVICE_DOCUMENTS_APP_NAME"),
				ResourceType: v.GetString("WORKSPACE_SERVICE_DOCUMENTS_RESOURCE_TYPE"),
			},
			Tasks: ResourceMapping{
				AppName:      v.GetString("WORKSPACE_SERVICE_TASKS_APP_NAME"),
				ResourceType: v.GetString("WORKSPACE_SERVICE_TASKS_RESOURCE_TYPE"),
			},
			Drive: ResourceMapping{
				AppName:      v.GetString("WORKSPACE_SERVICE_DRIVE_APP_NAME"),
				ResourceType: v.GetString("WORKSPACE_SERVICE_DRIVE_RESOURCE_TYPE"),
			},
		},
		CommandPublishTimeout: v.GetDuration("WORKSPACE_SERVICE_COMMAND_PUBLISH_TIMEOUT"),
		ShutdownTimeout:       v.GetDuration("WORKSPACE_SERVICE_SHUTDOWN_TIMEOUT"),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if !environment.IsValidEnvironment(c.Environment) {
		return fmt.Errorf("%w: WORKSPACE_SERVICE_ENV must be %q or %q", environment.ErrInvalidEnv, environment.Development, environment.Production)
	}
	required := map[string]string{
		"WORKSPACE_SERVICE_HTTP_ADDR":               c.HTTPAddr,
		"WORKSPACE_SERVICE_MONGODB_URI":             c.MongoDB.URI,
		"WORKSPACE_SERVICE_MONGODB_DATABASE":        c.MongoDB.Database,
		"WORKSPACE_SERVICE_NATS_URL":                c.NATS.URL,
		"WORKSPACE_SERVICE_HR_BASE_URL":             c.HR.BaseURL,
		"WORKSPACE_SERVICE_DOCUMENTS_APP_NAME":      c.ResourceMappings.Documents.AppName,
		"WORKSPACE_SERVICE_DOCUMENTS_RESOURCE_TYPE": c.ResourceMappings.Documents.ResourceType,
		"WORKSPACE_SERVICE_TASKS_APP_NAME":          c.ResourceMappings.Tasks.AppName,
		"WORKSPACE_SERVICE_TASKS_RESOURCE_TYPE":     c.ResourceMappings.Tasks.ResourceType,
		"WORKSPACE_SERVICE_DRIVE_APP_NAME":          c.ResourceMappings.Drive.AppName,
		"WORKSPACE_SERVICE_DRIVE_RESOURCE_TYPE":     c.ResourceMappings.Drive.ResourceType,
		"WORKSPACE_SERVICE_COMMAND_PUBLISH_TIMEOUT": c.CommandPublishTimeout.String(),
		"WORKSPACE_SERVICE_SHUTDOWN_TIMEOUT":        c.ShutdownTimeout.String(),
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	if err := validateBaseURL(c.HR.BaseURL); err != nil {
		return err
	}
	if err := validateResourceMapping("WORKSPACE_SERVICE_DOCUMENTS", c.ResourceMappings.Documents); err != nil {
		return err
	}
	if err := validateResourceMapping("WORKSPACE_SERVICE_TASKS", c.ResourceMappings.Tasks); err != nil {
		return err
	}
	if err := validateResourceMapping("WORKSPACE_SERVICE_DRIVE", c.ResourceMappings.Drive); err != nil {
		return err
	}
	if c.CommandPublishTimeout <= 0 {
		return fmt.Errorf("WORKSPACE_SERVICE_COMMAND_PUBLISH_TIMEOUT must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("WORKSPACE_SERVICE_SHUTDOWN_TIMEOUT must be positive")
	}
	return nil
}

func validateBaseURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("WORKSPACE_SERVICE_HR_BASE_URL must be valid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("WORKSPACE_SERVICE_HR_BASE_URL must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("WORKSPACE_SERVICE_HR_BASE_URL must include host")
	}
	return nil
}

func validateResourceMapping(prefix string, mapping ResourceMapping) error {
	if !isValidSubjectToken(mapping.AppName) {
		return fmt.Errorf("%s_APP_NAME must be a valid subject token", prefix)
	}
	if strings.TrimSpace(mapping.ResourceType) == "" {
		return fmt.Errorf("%s_RESOURCE_TYPE is required", prefix)
	}
	return nil
}

func isValidSubjectToken(value string) bool {
	return strings.TrimSpace(value) != "" && !strings.ContainsAny(value, ". \t\r\n")
}
