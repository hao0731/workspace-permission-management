package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Environment string

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentProduction  Environment = "production"
)

type Config struct {
	Environment     Environment
	HTTPAddr        string
	MongoDB         MongoDBConfig
	NATS            NATSConfig
	JetStream       JetStreamConfig
	ShutdownTimeout time.Duration
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
	Subject    string
	FetchCount int
	MaxWait    time.Duration
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()
	v.AutomaticEnv()

	v.SetDefault("FUNCTION_SERVICE_ENV", string(EnvironmentDevelopment))
	v.SetDefault("FUNCTION_SERVICE_JETSTREAM_FETCH_COUNT", 20)
	v.SetDefault("FUNCTION_SERVICE_JETSTREAM_MAX_WAIT", "5s")
	v.SetDefault("FUNCTION_SERVICE_SHUTDOWN_TIMEOUT", "10s")

	cfg := Config{
		Environment: Environment(v.GetString("FUNCTION_SERVICE_ENV")),
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
			Subject:    v.GetString("FUNCTION_SERVICE_JETSTREAM_SUBJECT"),
			FetchCount: v.GetInt("FUNCTION_SERVICE_JETSTREAM_FETCH_COUNT"),
			MaxWait:    v.GetDuration("FUNCTION_SERVICE_JETSTREAM_MAX_WAIT"),
		},
		ShutdownTimeout: v.GetDuration("FUNCTION_SERVICE_SHUTDOWN_TIMEOUT"),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.Environment != EnvironmentDevelopment && c.Environment != EnvironmentProduction {
		return fmt.Errorf("FUNCTION_SERVICE_ENV must be %q or %q", EnvironmentDevelopment, EnvironmentProduction)
	}

	required := map[string]string{
		"FUNCTION_SERVICE_HTTP_ADDR":         c.HTTPAddr,
		"FUNCTION_SERVICE_MONGODB_URI":       c.MongoDB.URI,
		"FUNCTION_SERVICE_MONGODB_DATABASE":  c.MongoDB.Database,
		"FUNCTION_SERVICE_NATS_URL":          c.NATS.URL,
		"FUNCTION_SERVICE_JETSTREAM_STREAM":  c.JetStream.Stream,
		"FUNCTION_SERVICE_JETSTREAM_DURABLE": c.JetStream.Durable,
		"FUNCTION_SERVICE_JETSTREAM_SUBJECT": c.JetStream.Subject,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	if c.JetStream.FetchCount <= 0 {
		return fmt.Errorf("FUNCTION_SERVICE_JETSTREAM_FETCH_COUNT must be greater than zero")
	}
	if c.JetStream.MaxWait <= 0 {
		return fmt.Errorf("FUNCTION_SERVICE_JETSTREAM_MAX_WAIT must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("FUNCTION_SERVICE_SHUTDOWN_TIMEOUT must be positive")
	}
	return nil
}
