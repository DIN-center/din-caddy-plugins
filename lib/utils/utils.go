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
)

// GetMachineId returns a unique string for the current running process
func GetMachineId() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "UNKNOWN"
	}
	currentPid := os.Getpid()
	return fmt.Sprintf("@%s:%d", hostname, currentPid)
}
