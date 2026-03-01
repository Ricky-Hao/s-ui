package config

import (
	"os"
	"testing"
)

func TestGetResetTrafficInterval_Default(t *testing.T) {
	os.Unsetenv("SUI_RESET_TRAFFIC_INTERVAL")
	result := GetResetTrafficInterval()
	expected := "@every 10m"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGetResetTrafficInterval_Custom(t *testing.T) {
	os.Setenv("SUI_RESET_TRAFFIC_INTERVAL", "5m")
	defer os.Unsetenv("SUI_RESET_TRAFFIC_INTERVAL")

	result := GetResetTrafficInterval()
	expected := "@every 5m"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGetResetTrafficInterval_CustomSeconds(t *testing.T) {
	os.Setenv("SUI_RESET_TRAFFIC_INTERVAL", "30s")
	defer os.Unsetenv("SUI_RESET_TRAFFIC_INTERVAL")

	result := GetResetTrafficInterval()
	expected := "@every 30s"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
