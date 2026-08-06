package transports

import "testing"

func TestDerefString(t *testing.T) {
	tests := []struct {
		name  string
		input *string
		want  string
	}{
		{"nil", nil, ""},
		{"empty", strPtr(""), ""},
		{"value", strPtr("hello"), "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := derefString(tt.input); got != tt.want {
				t.Errorf("derefString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDerefInt32(t *testing.T) {
	tests := []struct {
		name  string
		input *int32
		want  int32
	}{
		{"nil", nil, 0},
		{"zero", int32Ptr(0), 0},
		{"positive", int32Ptr(42), 42},
		{"negative", int32Ptr(-1), -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := derefInt32(tt.input); got != tt.want {
				t.Errorf("derefInt32() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestJsonStrToSmithyDoc(t *testing.T) {
	// Empty string should produce empty object.
	doc := jsonStrToSmithyDoc("")
	if doc == nil {
		t.Fatal("jsonStrToSmithyDoc(\"\") returned nil")
	}

	// Valid JSON.
	doc = jsonStrToSmithyDoc(`{"key":"value"}`)
	if doc == nil {
		t.Fatal("jsonStrToSmithyDoc(valid) returned nil")
	}
}

func TestToSmithyDoc_Nil(t *testing.T) {
	doc := toSmithyDoc(nil)
	if doc == nil {
		t.Fatal("toSmithyDoc(nil) returned nil")
	}
}

func strPtr(s string) *string { return &s }
func int32Ptr(i int32) *int32 { return &i }
