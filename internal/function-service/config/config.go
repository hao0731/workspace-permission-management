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
	Environment            environment.Environment
	HTTPAddr               string
	MongoDB                MongoDBConfig
	NATS                   NATSConfig
	JetStream              JetStreamConfig
	SystemResourceLimits   SystemResourceLimitsConfig
	PermissionAPI          PermissionAPIConfig
	ResourceDeletedSubject string
	ShutdownTimeout        time.Duration
}

type MongoDBConfig struct {
	URI      string
	Database string
}

type NATSConfig struct {
	URL string
}

type JetStreamConfig struct {
	Stream     string
	Durable    string
	FetchCount int
	MaxWait    time.Duration
}

type SystemResourceLimitsConfig struct {
	Type   int
	Action int
	Tag    int
}

type PermissionAPIConfig struct {
	BaseURL      string
	APIKey       string
	APIKeyHeader string
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()
	v.AutomaticEnv()

	v.SetDefault("FUNCTION_SERVICE_ENV", string(environment.Development))
	v.SetDefault("FUNCTION_SERVICE_JETSTREAM_FETCH_COUNT", 20)
	v.SetDefault("FUNCTION_SERVICE_JETSTREAM_MAX_WAIT", "5s")
	v.SetDefault("FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT", "app.todo.resource.deleted")
	v.SetDefault("FUNCTION_SERVICE_SYSTEM_RESOURCE_TYPE_LIMIT", 3)
	v.SetDefault("FUNCTION_SERVICE_SYSTEM_RESOURCE_ACTION_LIMIT", 5)
	v.SetDefault("FUNCTION_SERVICE_SYSTEM_RESOURCE_TAG_LIMIT", 20)
	v.SetDefault("FUNCTION_SERVICE_SHUTDOWN_TIMEOUT", "10s")

	cfg := Config{
		Environment: environment.Environment(v.GetString("FUNCTION_SERVICE_ENV")),
		HTTPAddr:    v.GetString("FUNCTION_SERVICE_HTTP_ADDR"),
		MongoDB: MongoDBConfig{
			URI:      v.GetString("FUNCTION_SERVICE_MONGODB_URI"),
			Database: v.GetString("FUNCTION_SERVICE_MONGODB_DATABASE"),
		},
		NATS: NATSConfig{
			URL: v.GetString("FUNCTION_SERVICE_NATS_URL"),
		},
		JetStream: JetStreamConfig{
			Stream:     v.GetString("FUNCTION_SERVICE_JETSTREAM_STREAM"),
			Durable:    v.GetString("FUNCTION_SERVICE_JETSTREAM_DURABLE"),
			FetchCount: v.GetInt("FUNCTION_SERVICE_JETSTREAM_FETCH_COUNT"),
			MaxWait:    v.GetDuration("FUNCTION_SERVICE_JETSTREAM_MAX_WAIT"),
		},
		SystemResourceLimits: SystemResourceLimitsConfig{
			Type:   v.GetInt("FUNCTION_SERVICE_SYSTEM_RESOURCE_TYPE_LIMIT"),
			Action: v.GetInt("FUNCTION_SERVICE_SYSTEM_RESOURCE_ACTION_LIMIT"),
			Tag:    v.GetInt("FUNCTION_SERVICE_SYSTEM_RESOURCE_TAG_LIMIT"),
		},
		PermissionAPI: PermissionAPIConfig{
			BaseURL:      v.GetString("FUNCTION_SERVICE_PERMISSION_API_BASE_URL"),
			APIKey:       v.GetString("FUNCTION_SERVICE_PERMISSION_API_KEY"),
			APIKeyHeader: v.GetString("FUNCTION_SERVICE_PERMISSION_API_KEY_HEADER"),
		},
		ResourceDeletedSubject: v.GetString("FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT"),
		ShutdownTimeout:        v.GetDuration("FUNCTION_SERVICE_SHUTDOWN_TIMEOUT"),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if !environment.IsValidEnvironment(c.Environment) {
		return fmt.Errorf("%w: FUNCTION_SERVICE_ENV must be %q or %q", environment.ErrInvalidEnv, environment.Development, environment.Production)
	}

	required := map[string]string{
		"FUNCTION_SERVICE_HTTP_ADDR":                 c.HTTPAddr,
		"FUNCTION_SERVICE_MONGODB_URI":               c.MongoDB.URI,
		"FUNCTION_SERVICE_MONGODB_DATABASE":          c.MongoDB.Database,
		"FUNCTION_SERVICE_NATS_URL":                  c.NATS.URL,
		"FUNCTION_SERVICE_JETSTREAM_STREAM":          c.JetStream.Stream,
		"FUNCTION_SERVICE_JETSTREAM_DURABLE":         c.JetStream.Durable,
		"FUNCTION_SERVICE_RESOURCE_DELETED_SUBJECT":  c.ResourceDeletedSubject,
		"FUNCTION_SERVICE_PERMISSION_API_BASE_URL":   c.PermissionAPI.BaseURL,
		"FUNCTION_SERVICE_PERMISSION_API_KEY":        c.PermissionAPI.APIKey,
		"FUNCTION_SERVICE_PERMISSION_API_KEY_HEADER": c.PermissionAPI.APIKeyHeader,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	if c.JetStream.FetchCount <= 0 {
		return fmt.Errorf("FUNCTION_SERVICE_JETSTREAM_FETCH_COUNT must be greater than zero")
	}
	if c.SystemResourceLimits.Type <= 0 {
		return fmt.Errorf("FUNCTION_SERVICE_SYSTEM_RESOURCE_TYPE_LIMIT must be greater than zero")
	}
	if c.SystemResourceLimits.Action <= 0 {
		return fmt.Errorf("FUNCTION_SERVICE_SYSTEM_RESOURCE_ACTION_LIMIT must be greater than zero")
	}
	if c.SystemResourceLimits.Tag <= 0 {
		return fmt.Errorf("FUNCTION_SERVICE_SYSTEM_RESOURCE_TAG_LIMIT must be greater than zero")
	}
	if err := validatePermissionAPIBaseURL(c.PermissionAPI.BaseURL); err != nil {
		return err
	}
	if !isHTTPHeaderName(strings.TrimSpace(c.PermissionAPI.APIKeyHeader)) {
		return fmt.Errorf("FUNCTION_SERVICE_PERMISSION_API_KEY_HEADER must be a valid HTTP header name")
	}
	if c.JetStream.MaxWait <= 0 {
		return fmt.Errorf("FUNCTION_SERVICE_JETSTREAM_MAX_WAIT must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("FUNCTION_SERVICE_SHUTDOWN_TIMEOUT must be positive")
	}
	return nil
}

func validatePermissionAPIBaseURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("FUNCTION_SERVICE_PERMISSION_API_BASE_URL must be an absolute http or https URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("FUNCTION_SERVICE_PERMISSION_API_BASE_URL must be an absolute http or https URL")
	}
	return nil
}

func isHTTPHeaderName(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if strings.ContainsRune("!#$%&'*+-.^_`|~", r) {
			continue
		}
		return false
	}
	return true
}
