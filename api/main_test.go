package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestParseJobStatusPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		wantID  string
		wantErr bool
	}{
		{
			name:    "valid",
			path:    "/v1/jobs/6aab8fca-7059-40c4-97d4-53f55fd5bf67/status",
			wantID:  "6aab8fca-7059-40c4-97d4-53f55fd5bf67",
			wantErr: false,
		},
		{
			name:    "missing status suffix",
			path:    "/v1/jobs/6aab8fca-7059-40c4-97d4-53f55fd5bf67",
			wantErr: true,
		},
		{
			name:    "wrong prefix",
			path:    "/v2/jobs/6aab8fca-7059-40c4-97d4-53f55fd5bf67/status",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotID, err := parseJobStatusPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseJobStatusPath() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && gotID != tt.wantID {
				t.Fatalf("parseJobStatusPath() id = %s, want %s", gotID, tt.wantID)
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

func TestApplyCORSHeaders_WithOrigin(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()

	applyCORSHeaders(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("allow-origin = %q, want %q", got, "http://localhost:5173")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != corsAllowMethods {
		t.Fatalf("allow-methods = %q, want %q", got, corsAllowMethods)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != corsAllowHeaders {
		t.Fatalf("allow-headers = %q, want %q", got, corsAllowHeaders)
	}
}
