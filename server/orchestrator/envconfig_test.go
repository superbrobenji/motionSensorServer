package main

import (
	"os"
	"testing"
)

func TestEnvOrDefault_StringUsesEnv(t *testing.T) {
	os.Setenv("TEST_VAR", "from-env")
	defer os.Unsetenv("TEST_VAR")
	result := envOrDefault("TEST_VAR", "default")
	if result != "from-env" {
		t.Errorf("expected 'from-env', got %q", result)
	}
}

func TestEnvOrDefault_StringFallsBack(t *testing.T) {
	os.Unsetenv("TEST_VAR")
	result := envOrDefault("TEST_VAR", "default")
	if result != "default" {
		t.Errorf("expected 'default', got %q", result)
	}
}

func TestEnvOrDefaultInt_UsesEnv(t *testing.T) {
	os.Setenv("TEST_INT", "9999")
	defer os.Unsetenv("TEST_INT")
	result := envOrDefaultInt("TEST_INT", 1234)
	if result != 9999 {
		t.Errorf("expected 9999, got %d", result)
	}
}

func TestEnvOrDefaultInt_FallsBack(t *testing.T) {
	os.Unsetenv("TEST_INT")
	result := envOrDefaultInt("TEST_INT", 1234)
	if result != 1234 {
		t.Errorf("expected 1234, got %d", result)
	}
}
