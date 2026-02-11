package utils

import (
	"fmt"
	"os"
	"slices"
)

// Environment represents the deployment environment
type Environment string

const (
	// Environment Constants
	EnvProd    Environment = "prod"
	EnvStaging Environment = "staging"
	EnvBeta    Environment = "beta"
	EnvDev     Environment = "dev"
	EnvTest    Environment = "test"
	EnvUnknown Environment = "unknown"
)

var AvailableEnvironments = []Environment{EnvProd, EnvStaging, EnvBeta, EnvDev, EnvTest}

// GetEnv returns the environment variable
func GetEnv() Environment {
	env := Environment(os.Getenv("ENV"))
	if !slices.Contains(AvailableEnvironments, env) {
		env = EnvUnknown
	}
	return env
}

// GetMachineId returns a unique string for the current running process
func GetMachineId() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "UNKNOWN"
	}
	currentPid := os.Getpid()
	return fmt.Sprintf("@%s:%d", hostname, currentPid)
}
