package modules

import (
	"testing"
	"time"
)

func TestDinMiddleware_RetryDefaultInitialization(t *testing.T) {
	// Create a new DinMiddleware instance without setting retry config
	d := &DinMiddleware{}
	
	// Call initializeDefaults
	d.initializeDefaults()
	
	// Verify default retry values are set
	if d.RegistryRetryMaxAttempts != DefaultRegistryRetryMaxAttempts {
		t.Errorf("Expected RegistryRetryMaxAttempts to be %d, got %d", 
			DefaultRegistryRetryMaxAttempts, d.RegistryRetryMaxAttempts)
	}
	
	expectedInitialDelay, _ := time.ParseDuration(DefaultRegistryRetryInitialDelay)
	if d.RegistryRetryInitialDelay != expectedInitialDelay {
		t.Errorf("Expected RegistryRetryInitialDelay to be %v, got %v", 
			expectedInitialDelay, d.RegistryRetryInitialDelay)
	}
	
	expectedMaxDelay, _ := time.ParseDuration(DefaultRegistryRetryMaxDelay)
	if d.RegistryRetryMaxDelay != expectedMaxDelay {
		t.Errorf("Expected RegistryRetryMaxDelay to be %v, got %v", 
			expectedMaxDelay, d.RegistryRetryMaxDelay)
	}
	
	if d.RegistryRetryBackoffFactor != DefaultRegistryRetryBackoffFactor {
		t.Errorf("Expected RegistryRetryBackoffFactor to be %f, got %f", 
			DefaultRegistryRetryBackoffFactor, d.RegistryRetryBackoffFactor)
	}
}

func TestDinMiddleware_RetryPartialConfiguration(t *testing.T) {
	// Create a DinMiddleware with only some retry values set
	d := &DinMiddleware{
		RegistryRetryMaxAttempts: 5, // Custom value
		// Leave other retry fields at zero values
	}
	
	// Call initializeDefaults
	d.initializeDefaults()
	
	// Verify custom value is preserved
	if d.RegistryRetryMaxAttempts != 5 {
		t.Errorf("Expected RegistryRetryMaxAttempts to remain 5, got %d", 
			d.RegistryRetryMaxAttempts)
	}
	
	// Verify other defaults are set
	expectedInitialDelay, _ := time.ParseDuration(DefaultRegistryRetryInitialDelay)
	if d.RegistryRetryInitialDelay != expectedInitialDelay {
		t.Errorf("Expected RegistryRetryInitialDelay to be %v, got %v", 
			expectedInitialDelay, d.RegistryRetryInitialDelay)
	}
	
	expectedMaxDelay, _ := time.ParseDuration(DefaultRegistryRetryMaxDelay)
	if d.RegistryRetryMaxDelay != expectedMaxDelay {
		t.Errorf("Expected RegistryRetryMaxDelay to be %v, got %v", 
			expectedMaxDelay, d.RegistryRetryMaxDelay)
	}
	
	if d.RegistryRetryBackoffFactor != DefaultRegistryRetryBackoffFactor {
		t.Errorf("Expected RegistryRetryBackoffFactor to be %f, got %f", 
			DefaultRegistryRetryBackoffFactor, d.RegistryRetryBackoffFactor)
	}
}

func TestDinMiddleware_RetryFullConfiguration(t *testing.T) {
	// Create a DinMiddleware with all retry values set
	customInitialDelay := 2 * time.Second
	customMaxDelay := 60 * time.Second
	
	d := &DinMiddleware{
		RegistryRetryMaxAttempts:   10,
		RegistryRetryInitialDelay:  customInitialDelay,
		RegistryRetryMaxDelay:      customMaxDelay,
		RegistryRetryBackoffFactor: 3.0,
	}
	
	// Call initializeDefaults
	d.initializeDefaults()
	
	// Verify all custom values are preserved
	if d.RegistryRetryMaxAttempts != 10 {
		t.Errorf("Expected RegistryRetryMaxAttempts to remain 10, got %d", 
			d.RegistryRetryMaxAttempts)
	}
	
	if d.RegistryRetryInitialDelay != customInitialDelay {
		t.Errorf("Expected RegistryRetryInitialDelay to remain %v, got %v", 
			customInitialDelay, d.RegistryRetryInitialDelay)
	}
	
	if d.RegistryRetryMaxDelay != customMaxDelay {
		t.Errorf("Expected RegistryRetryMaxDelay to remain %v, got %v", 
			customMaxDelay, d.RegistryRetryMaxDelay)
	}
	
	if d.RegistryRetryBackoffFactor != 3.0 {
		t.Errorf("Expected RegistryRetryBackoffFactor to remain 3.0, got %f", 
			d.RegistryRetryBackoffFactor)
	}
}