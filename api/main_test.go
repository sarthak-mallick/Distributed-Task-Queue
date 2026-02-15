package main

import (
	"encoding/json"
	"testing"
)

func TestValidateSubmitJobRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   submitJobRequest
		wantErr bool
	}{
		{
			name: "valid weather job",
			input: submitJobRequest{
				JobType: "weather",
				Payload: mustJSON(t, map[string]any{
					"city":         "Austin",
					"country_code": "US",
					"units":        "metric",
				}),
			},
			wantErr: false,
		},
		{
			name: "valid quote job",
			input: submitJobRequest{
				JobType: "quote",
				Payload: mustJSON(t, map[string]any{
					"author": "Einstein",
				}),
			},
			wantErr: false,
		},
		{
			name: "unsupported job type",
			input: submitJobRequest{
				JobType: "custom",
				Payload: mustJSON(t, map[string]any{"k": "v"}),
			},
			wantErr: true,
		},
		{
			name: "missing payload",
			input: submitJobRequest{
				JobType: "weather",
			},
			wantErr: true,
		},
		{
			name: "invalid weather payload",
			input: submitJobRequest{
				JobType: "weather",
				Payload: mustJSON(t, map[string]any{
					"city":  "Austin",
					"units": "kelvin",
				}),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := validateSubmitJobRequest(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateSubmitJobRequest() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	return b
}

