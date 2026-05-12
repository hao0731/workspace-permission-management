package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

const resourceCreateConsumerSubject = "cmd.app.*.resource.create"

type Config struct {
	Environment                  environment.Environment
	HTTPAddr                     string
	NATS                         NATSConfig
	ResourceCreate               ResourceCreateConfig
	AppNames                     AppNamesConfig
	ResourceUpsertPublishTimeout time.Duration
	ShutdownTimeout              time.Duration
}

type NATSConfig struct {
	URL string
}

type ResourceCreateConfig struct {
	Stream     string
	Durable    string
	FetchCount int
	MaxWait    time.Duration
}

type AppNamesConfig struct {
	Documents string
	Tasks     string
	Drive     string
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()
	v.AutomaticEnv()

	v.SetDefault("MOCK_FUNCTION_ENV", string(environment.Development))
	v.SetDefault("MOCK_FUNCTION_RESOURCE_CREATE_FETCH_COUNT", 20)
	v.SetDefault("MOCK_FUNCTION_RESOURCE_CREATE_MAX_WAIT", "5s")
	v.SetDefault("MOCK_FUNCTION_RESOURCE_UPSERT_PUBLISH_TIMEOUT", "15s")
	v.SetDefault("MOCK_FUNCTION_SHUTDOWN_TIMEOUT", "10s")

	cfg := Config{
		Environment: environment.Environment(v.GetString("MOCK_FUNCTION_ENV")),
		HTTPAddr:    v.GetString("MOCK_FUNCTION_HTTP_ADDR"),
		NATS: NATSConfig{
			URL: v.GetString("MOCK_FUNCTION_NATS_URL"),
		},
		ResourceCreate: ResourceCreateConfig{
			Stream:     v.GetString("MOCK_FUNCTION_RESOURCE_CREATE_STREAM"),
			Durable:    v.GetString("MOCK_FUNCTION_RESOURCE_CREATE_DURABLE"),
			FetchCount: v.GetInt("MOCK_FUNCTION_RESOURCE_CREATE_FETCH_COUNT"),
			MaxWait:    v.GetDuration("MOCK_FUNCTION_RESOURCE_CREATE_MAX_WAIT"),
		},
		AppNames: AppNamesConfig{
			Documents: v.GetString("MOCK_FUNCTION_DOCUMENTS_APP_NAME"),
			Tasks:     v.GetString("MOCK_FUNCTION_TASKS_APP_NAME"),
			Drive:     v.GetString("MOCK_FUNCTION_DRIVE_APP_NAME"),
		},
		ResourceUpsertPublishTimeout: v.GetDuration("MOCK_FUNCTION_RESOURCE_UPSERT_PUBLISH_TIMEOUT"),
		ShutdownTimeout:              v.GetDuration("MOCK_FUNCTION_SHUTDOWN_TIMEOUT"),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if !environment.IsValidEnvironment(c.Environment) {
		return fmt.Errorf("%w: MOCK_FUNCTION_ENV must be %q or %q", environment.ErrInvalidEnv, environment.Development, environment.Production)
	}
	required := map[string]string{
		"MOCK_FUNCTION_HTTP_ADDR":                c.HTTPAddr,
		"MOCK_FUNCTION_NATS_URL":                 c.NATS.URL,
		"MOCK_FUNCTION_RESOURCE_CREATE_STREAM":   c.ResourceCreate.Stream,
		"MOCK_FUNCTION_RESOURCE_CREATE_DURABLE":  c.ResourceCreate.Durable,
		"MOCK_FUNCTION_DOCUMENTS_APP_NAME":       c.AppNames.Documents,
		"MOCK_FUNCTION_TASKS_APP_NAME":           c.AppNames.Tasks,
		"MOCK_FUNCTION_DRIVE_APP_NAME":           c.AppNames.Drive,
		"MOCK_FUNCTION_RESOURCE_CREATE_MAX_WAIT": c.ResourceCreate.MaxWait.String(),
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	if err := validateAppName("MOCK_FUNCTION_DOCUMENTS_APP_NAME", c.AppNames.Documents); err != nil {
		return err
	}
	if err := validateAppName("MOCK_FUNCTION_TASKS_APP_NAME", c.AppNames.Tasks); err != nil {
		return err
	}
	if err := validateAppName("MOCK_FUNCTION_DRIVE_APP_NAME", c.AppNames.Drive); err != nil {
		return err
	}
	if c.ResourceCreate.FetchCount <= 0 {
		return fmt.Errorf("MOCK_FUNCTION_RESOURCE_CREATE_FETCH_COUNT must be greater than zero")
	}
	if c.ResourceCreate.MaxWait <= 0 {
		return fmt.Errorf("MOCK_FUNCTION_RESOURCE_CREATE_MAX_WAIT must be positive")
	}
	if c.ResourceUpsertPublishTimeout <= 0 {
		return fmt.Errorf("MOCK_FUNCTION_RESOURCE_UPSERT_PUBLISH_TIMEOUT must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("MOCK_FUNCTION_SHUTDOWN_TIMEOUT must be positive")
	}
	return nil
}

func (c Config) ResourceCreateSubjectAppNames() map[string]string {
	return map[string]string{
		resourceCreateSubject(c.AppNames.Documents): c.AppNames.Documents,
		resourceCreateSubject(c.AppNames.Tasks):     c.AppNames.Tasks,
		resourceCreateSubject(c.AppNames.Drive):     c.AppNames.Drive,
	}
}

func (c Config) ResourceCreateConsumerSubject() string {
	return resourceCreateConsumerSubject
}

func resourceCreateSubject(appName string) string {
	return fmt.Sprintf("cmd.app.%s.resource.create", appName)
}

func validateAppName(key string, value string) error {
	if strings.TrimSpace(value) == "" || strings.ContainsAny(value, ". \t\r\n") {
		return fmt.Errorf("%s must be a valid subject token", key)
	}
	return nil
}
