package utils

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	type testCase struct {
		name     string
		envValue string
		expect   Environment
	}

	// In order to avoid the test cases affecting the actual ENV variable, save the original ENV variable and restore it after the test
	originalEnv := GetEnv()
	originalEnvString := string(originalEnv)
	defer func() {
		_ = os.Setenv("ENV", originalEnvString)
	}()

	testCases := []testCase{
		{"Recognized prod env", "prod", EnvProd},
		{"Recognized staging env", "staging", EnvStaging},
		{"Recognized beta env", "beta", EnvBeta},
		{"Recognized dev env", "dev", EnvDev},
		{"Recognized test env", "test", EnvTest},
		{"Unrecognized env value", "foobar", EnvUnknown},
		{"Empty env value", "", EnvUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.Setenv("ENV", tc.envValue); err != nil {
				t.Fatalf("failed to set ENV for test: %v", err)
			}
			actual := GetEnv()
			if actual != tc.expect {
				t.Errorf("GetEnv() with ENV=%q returned %q; expected %q", tc.envValue, actual, tc.expect)
			}
		})
	}
}
