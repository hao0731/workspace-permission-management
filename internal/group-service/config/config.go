package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

type Config struct {
	Environment                   environment.Environment
	HTTPAddr                      string
	MongoDB                       MongoDBConfig
	NATS                          NATSConfig
	Validation                    ValidationConfig
	GroupExpiryCommand            GroupExpiryCommandConfig
	IndividualMemberExpiryCommand IndividualMemberExpiryCommandConfig
	ShutdownTimeout               time.Duration
}

type MongoDBConfig struct {
	URI      string
	Database string
}

type NATSConfig struct {
	URL string
}

type ValidationConfig struct {
	MaxIndividualMembers int
	MaxGroupingRules     int
}

type GroupExpiryCommandConfig struct {
	Stream         string
	Durable        string
	Subject        string
	FetchCount     int
	MaxWait        time.Duration
	BucketTimezone string
	BucketLocation *time.Location
}

type IndividualMemberExpiryCommandConfig struct {
	Stream         string
	Durable        string
	Subject        string
	FetchCount     int
	MaxWait        time.Duration
	BucketTimezone string
	BucketLocation *time.Location
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
	v.SetDefault("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_FETCH_COUNT", 20)
	v.SetDefault("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_MAX_WAIT", "5s")
	v.SetDefault("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE", "UTC")
	v.SetDefault("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_FETCH_COUNT", 20)
	v.SetDefault("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_MAX_WAIT", "5s")
	v.SetDefault("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE", "UTC")

	bucketTimezone := v.GetString("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE")
	bucketLocation, err := parseExpirationBucketLocationConfig("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE", bucketTimezone)
	if err != nil {
		return Config{}, err
	}
	memberBucketTimezone := v.GetString("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE")
	memberBucketLocation, err := parseExpirationBucketLocationConfig("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE", memberBucketTimezone)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Environment: environment.Environment(v.GetString("GROUP_SERVICE_ENV")),
		HTTPAddr:    v.GetString("GROUP_SERVICE_HTTP_ADDR"),
		MongoDB: MongoDBConfig{
			URI:      v.GetString("GROUP_SERVICE_MONGODB_URI"),
			Database: v.GetString("GROUP_SERVICE_MONGODB_DATABASE"),
		},
		NATS: NATSConfig{
			URL: v.GetString("GROUP_SERVICE_NATS_URL"),
		},
		Validation: ValidationConfig{
			MaxIndividualMembers: v.GetInt("GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS"),
			MaxGroupingRules:     v.GetInt("GROUP_SERVICE_MAX_GROUPING_RULES"),
		},
		GroupExpiryCommand: GroupExpiryCommandConfig{
			Stream:         v.GetString("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_STREAM"),
			Durable:        v.GetString("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_DURABLE"),
			Subject:        v.GetString("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT"),
			FetchCount:     v.GetInt("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_FETCH_COUNT"),
			MaxWait:        v.GetDuration("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_MAX_WAIT"),
			BucketTimezone: bucketTimezone,
			BucketLocation: bucketLocation,
		},
		IndividualMemberExpiryCommand: IndividualMemberExpiryCommandConfig{
			Stream:         v.GetString("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM"),
			Durable:        v.GetString("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE"),
			Subject:        v.GetString("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT"),
			FetchCount:     v.GetInt("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_FETCH_COUNT"),
			MaxWait:        v.GetDuration("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_MAX_WAIT"),
			BucketTimezone: memberBucketTimezone,
			BucketLocation: memberBucketLocation,
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
		"GROUP_SERVICE_HTTP_ADDR":                                c.HTTPAddr,
		"GROUP_SERVICE_MONGODB_URI":                              c.MongoDB.URI,
		"GROUP_SERVICE_MONGODB_DATABASE":                         c.MongoDB.Database,
		"GROUP_SERVICE_NATS_URL":                                 c.NATS.URL,
		"GROUP_SERVICE_GROUP_EXPIRY_COMMAND_STREAM":              c.GroupExpiryCommand.Stream,
		"GROUP_SERVICE_GROUP_EXPIRY_COMMAND_DURABLE":             c.GroupExpiryCommand.Durable,
		"GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT":             c.GroupExpiryCommand.Subject,
		"GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE":             c.GroupExpiryCommand.BucketTimezone,
		"GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_STREAM":  c.IndividualMemberExpiryCommand.Stream,
		"GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_DURABLE": c.IndividualMemberExpiryCommand.Durable,
		"GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT": c.IndividualMemberExpiryCommand.Subject,
		"GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE": c.IndividualMemberExpiryCommand.BucketTimezone,
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
	if c.GroupExpiryCommand.FetchCount <= 0 {
		return fmt.Errorf("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_FETCH_COUNT must be greater than zero")
	}
	if c.GroupExpiryCommand.MaxWait <= 0 {
		return fmt.Errorf("GROUP_SERVICE_GROUP_EXPIRY_COMMAND_MAX_WAIT must be positive")
	}
	if c.GroupExpiryCommand.BucketLocation == nil {
		return fmt.Errorf("GROUP_SERVICE_GROUP_EXPIRY_BUCKET_TIMEZONE must be valid")
	}
	if c.IndividualMemberExpiryCommand.FetchCount <= 0 {
		return fmt.Errorf("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_FETCH_COUNT must be greater than zero")
	}
	if c.IndividualMemberExpiryCommand.MaxWait <= 0 {
		return fmt.Errorf("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_MAX_WAIT must be positive")
	}
	if c.IndividualMemberExpiryCommand.BucketLocation == nil {
		return fmt.Errorf("GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE must be valid")
	}
	return nil
}

func parseExpirationBucketLocationConfig(key string, value string) (*time.Location, error) {
	location, err := group.ParseExpirationBucketLocation(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be valid: %w", key, err)
	}
	return location, nil
}
