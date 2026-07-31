package observability

import (
	"context"
	"os"
	"testing"
)

func TestInitTracer_NoEndpoint(t *testing.T) {
	// When OTEL_EXPORTER_OTLP_ENDPOINT is not set, InitTracer should return
	// a noop shutdown function without error.
	origEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	defer os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", origEndpoint)

	shutdown, err := InitTracer(context.Background(), "test-service", "0.1.0")
	if err != nil {
		t.Fatalf("InitTracer: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown function is nil")
	}
	// Shutdown should not error.
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestInitTracer_ServiceNameOverride(t *testing.T) {
	// OTEL_SERVICE_NAME should override the passed serviceName.
	// With no endpoint set, this just verifies the override path doesn't panic.
	origServiceName := os.Getenv("OTEL_SERVICE_NAME")
	origEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	os.Setenv("OTEL_SERVICE_NAME", "custom-service")
	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	defer os.Setenv("OTEL_SERVICE_NAME", origServiceName)
	defer os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", origEndpoint)

	shutdown, err := InitTracer(context.Background(), "test-service", "0.1.0")
	if err != nil {
		t.Fatalf("InitTracer: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown function is nil")
	}
}
