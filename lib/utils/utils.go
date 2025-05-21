package utils

import (
	"fmt"
	"os"
)

// Environment represents the deployment environment
type Environment string

const (
	// Environment Constants
	EnvProd Environment = "prod"
	EnvBeta Environment = "beta"
	EnvDev  Environment = "dev"
	EnvTest Environment = "test"
)

// GetEnv returns the environment variable
func GetEnv() Environment {
	env := Environment(os.Getenv("ENV"))
	if env != EnvProd && env != EnvBeta && env != EnvDev && env != EnvTest {
		env = EnvDev
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
