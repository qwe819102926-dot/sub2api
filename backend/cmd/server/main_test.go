package main

import (
	"testing"
	"time"
)

func TestShutdownTimeout(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want time.Duration
	}{
		{name: "default", want: 120 * time.Second},
		{name: "configured", env: "45s", want: 45 * time.Second},
		{name: "invalid", env: "not-a-duration", want: 120 * time.Second},
		{name: "non-positive", env: "0s", want: 120 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SERVER_SHUTDOWN_TIMEOUT", tt.env)
			if got := shutdownTimeout(); got != tt.want {
				t.Fatalf("shutdownTimeout() = %s, want %s", got, tt.want)
			}
		})
	}
}
