package agent

import "testing"

func TestCheckContextPressure(t *testing.T) {
	tests := []struct {
		used, ctx int
		want      ContextPressureLevel
	}{
		{0, 128000, PressureNone},
		{50000, 128000, PressureNone},
		{70000, 128000, PressureLow},
		{95000, 128000, PressureMedium},
		{115000, 128000, PressureHigh},
		{125000, 128000, PressureCritical},
		{100, 0, PressureNone},
	}

	for _, tt := range tests {
		got := CheckContextPressure(tt.used, tt.ctx)
		if got != tt.want {
			t.Errorf("CheckContextPressure(%d, %d) = %d, want %d", tt.used, tt.ctx, got, tt.want)
		}
	}
}

func TestLogContextPressure(t *testing.T) {
	if LogContextPressure(50000, 128000, "test") {
		t.Error("Expected no warning for low usage")
	}
	if !LogContextPressure(125000, 128000, "test") {
		t.Error("Expected warning for critical usage")
	}
}
