package environment

import "errors"

type Environment string

const (
	Development Environment = "development"
	Production  Environment = "production"
)

var ErrInvalidEnv = errors.New("invalid environment")

func IsValidEnvironment(env Environment) bool {
	return env == Development || env == Production
}
