package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/spf13/viper"

	"github.com/hao0731/workspace-permission-management/internal/domain/group"
	"github.com/hao0731/workspace-permission-management/internal/shared/environment"
)

type Config struct {
	Environment            environment.Environment
	HTTPAddr               string
	MongoDB                MongoDBConfig
	NATS                   NATSConfig
	GroupExpiry            ExpiryCommandConfig
	IndividualMemberExpiry ExpiryCommandConfig
	Schedule               ScheduleConfig
	BatchSize              int
	PublishTimeout         time.Duration
	ShutdownTimeout        time.Duration
}

type MongoDBConfig struct {
	URI      string
	Database string
}

type NATSConfig struct {
	URL string
}

type ExpiryCommandConfig struct {
	Subject        string
	BucketTimezone string
	BucketLocation *time.Location
}

type ScheduleConfig struct {
	Expression  string
	WithSeconds bool
	Timezone    string
	Location    *time.Location
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()
	v.AutomaticEnv()

	v.SetDefault("GROUP_EXPIRY_SCHEDULER_ENV", string(environment.Development))
	v.SetDefault("GROUP_EXPIRY_SCHEDULER_SHUTDOWN_TIMEOUT", "10s")
	v.SetDefault("GROUP_EXPIRY_SCHEDULER_BATCH_SIZE", 20)
	v.SetDefault("GROUP_EXPIRY_SCHEDULER_PUBLISH_TIMEOUT", "15s")
	v.SetDefault("GROUP_EXPIRY_SCHEDULER_CRON_WITH_SECONDS", false)
	v.SetDefault("GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE", "UTC")
	v.SetDefault("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE", "UTC")
	v.SetDefault("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE", "UTC")

	scheduleTimezone := v.GetString("GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE")
	scheduleLocation, err := parseScheduleLocation(scheduleTimezone)
	if err != nil {
		return Config{}, err
	}
	groupBucketTimezone := v.GetString("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE")
	groupBucketLocation, err := parseBucketLocation("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE", groupBucketTimezone)
	if err != nil {
		return Config{}, err
	}
	memberBucketTimezone := v.GetString("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE")
	memberBucketLocation, err := parseBucketLocation("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE", memberBucketTimezone)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Environment: environment.Environment(v.GetString("GROUP_EXPIRY_SCHEDULER_ENV")),
		HTTPAddr:    v.GetString("GROUP_EXPIRY_SCHEDULER_HTTP_ADDR"),
		MongoDB: MongoDBConfig{
			URI:      v.GetString("GROUP_EXPIRY_SCHEDULER_MONGODB_URI"),
			Database: v.GetString("GROUP_EXPIRY_SCHEDULER_MONGODB_DATABASE"),
		},
		NATS: NATSConfig{
			URL: v.GetString("GROUP_EXPIRY_SCHEDULER_NATS_URL"),
		},
		GroupExpiry: ExpiryCommandConfig{
			Subject:        v.GetString("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_COMMAND_SUBJECT"),
			BucketTimezone: groupBucketTimezone,
			BucketLocation: groupBucketLocation,
		},
		IndividualMemberExpiry: ExpiryCommandConfig{
			Subject:        v.GetString("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT"),
			BucketTimezone: memberBucketTimezone,
			BucketLocation: memberBucketLocation,
		},
		Schedule: ScheduleConfig{
			Expression:  v.GetString("GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION"),
			WithSeconds: v.GetBool("GROUP_EXPIRY_SCHEDULER_CRON_WITH_SECONDS"),
			Timezone:    scheduleTimezone,
			Location:    scheduleLocation,
		},
		BatchSize:       v.GetInt("GROUP_EXPIRY_SCHEDULER_BATCH_SIZE"),
		PublishTimeout:  v.GetDuration("GROUP_EXPIRY_SCHEDULER_PUBLISH_TIMEOUT"),
		ShutdownTimeout: v.GetDuration("GROUP_EXPIRY_SCHEDULER_SHUTDOWN_TIMEOUT"),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if !environment.IsValidEnvironment(c.Environment) {
		return fmt.Errorf("%w: GROUP_EXPIRY_SCHEDULER_ENV must be %q or %q", environment.ErrInvalidEnv, environment.Development, environment.Production)
	}
	required := map[string]string{
		"GROUP_EXPIRY_SCHEDULER_HTTP_ADDR":                                c.HTTPAddr,
		"GROUP_EXPIRY_SCHEDULER_MONGODB_URI":                              c.MongoDB.URI,
		"GROUP_EXPIRY_SCHEDULER_MONGODB_DATABASE":                         c.MongoDB.Database,
		"GROUP_EXPIRY_SCHEDULER_NATS_URL":                                 c.NATS.URL,
		"GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_COMMAND_SUBJECT":             c.GroupExpiry.Subject,
		"GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT": c.IndividualMemberExpiry.Subject,
		"GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION":                          c.Schedule.Expression,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	if c.BatchSize <= 0 {
		return fmt.Errorf("GROUP_EXPIRY_SCHEDULER_BATCH_SIZE must be greater than zero")
	}
	if c.PublishTimeout <= 0 {
		return fmt.Errorf("GROUP_EXPIRY_SCHEDULER_PUBLISH_TIMEOUT must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("GROUP_EXPIRY_SCHEDULER_SHUTDOWN_TIMEOUT must be positive")
	}
	if c.Schedule.Location == nil {
		return fmt.Errorf("GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE must be valid")
	}
	cron := gocron.NewDefaultCron(c.Schedule.WithSeconds)
	if err := cron.IsValid(c.Schedule.Expression, c.Schedule.Location, time.Now().In(c.Schedule.Location)); err != nil {
		return fmt.Errorf("GROUP_EXPIRY_SCHEDULER_CRON_EXPRESSION must be valid: %w", err)
	}
	if c.GroupExpiry.BucketLocation == nil {
		return fmt.Errorf("GROUP_EXPIRY_SCHEDULER_GROUP_EXPIRY_BUCKET_TIMEZONE must be valid")
	}
	if c.IndividualMemberExpiry.BucketLocation == nil {
		return fmt.Errorf("GROUP_EXPIRY_SCHEDULER_INDIVIDUAL_MEMBER_EXPIRY_BUCKET_TIMEZONE must be valid")
	}
	return nil
}

func parseScheduleLocation(value string) (*time.Location, error) {
	location, err := time.LoadLocation(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("GROUP_EXPIRY_SCHEDULER_SCHEDULER_TIMEZONE must be valid: %w", err)
	}
	return location, nil
}

func parseBucketLocation(key string, value string) (*time.Location, error) {
	location, err := group.ParseExpirationBucketLocation(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be valid: %w", key, err)
	}
	return location, nil
}
