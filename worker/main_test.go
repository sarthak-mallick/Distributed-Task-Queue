package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

// fakeConsumer is a test double for commit assertions.
type fakeConsumer struct {
	commitCalls int
	events      *[]string
}

// FetchMessage is unused in these unit tests.
func (f *fakeConsumer) FetchMessage(context.Context) (kafka.Message, error) {
	return kafka.Message{}, errors.New("not implemented")
}

// CommitMessages records commit invocations for assertions.
func (f *fakeConsumer) CommitMessages(_ context.Context, _ ...kafka.Message) error {
	f.commitCalls++
	if f.events != nil {
		*f.events = append(*f.events, "commit")
	}
	return nil
}

// Close satisfies the kafkaConsumer interface.
func (f *fakeConsumer) Close() error {
	return nil
}

// noOpStatusStore is a status store stub for process tests.
type noOpStatusStore struct{}

// UpsertStatus accepts status writes without side effects.
func (s noOpStatusStore) UpsertStatus(context.Context, jobStatusRecord) error {
	return nil
}

// noOpResultStore is a result store stub for process tests.
type noOpResultStore struct{}

// UpsertJobResult accepts result writes without side effects.
func (s noOpResultStore) UpsertJobResult(context.Context, jobResultDocument) (bool, error) {
	return true, nil
}

// weatherFetchResult scripts fake weather client responses for retry tests.
type weatherFetchResult struct {
	obs weatherObservation
	err error
}

// fakeWeatherClient is a scripted weather client test double.
type fakeWeatherClient struct {
	results []weatherFetchResult
	calls   int
}

// Fetch returns scripted results to validate retry/backoff behavior.
func (f *fakeWeatherClient) Fetch(context.Context, weatherPayload) (weatherObservation, error) {
	if f.calls >= len(f.results) {
		f.calls++
		return weatherObservation{}, errors.New("no scripted weather result")
	}
	result := f.results[f.calls]
	f.calls++
	return result.obs, result.err
}

// TestProcessFetchedMessageCommitsAfterSuccessfulHandler verifies deferred commit ordering.
func TestProcessFetchedMessageCommitsAfterSuccessfulHandler(t *testing.T) {
	t.Parallel()

	events := make([]string, 0, 2)
	consumer := &fakeConsumer{events: &events}
	w := &worker{
		cfg: config{
			processTimeout: time.Second,
			commitTimeout:  time.Second,
		},
		logger:      log.New(testWriter{t: t}, "", 0),
		consumer:    consumer,
		statusStore: noOpStatusStore{},
		resultStore: noOpResultStore{},
		handlers:    map[string]jobHandler{},
	}
	w.handlers["weather"] = func(context.Context, kafkaJobMessage) (jobExecutionResult, error) {
		events = append(events, "handler")
		return jobExecutionResult{
			Input:   map[string]any{"city": "Austin"},
			Output:  map[string]any{"temperature": 21.2},
			Message: "ok",
		}, nil
	}

	msg := kafka.Message{
		Topic:     "jobs.weather.v1",
		Partition: 0,
		Offset:    42,
		Key:       []byte("job-1"),
		Value: mustKafkaMessage(t, kafkaJobMessage{
			SchemaVersion: "1.0",
			JobID:         "job-1",
			JobType:       "weather",
			SubmittedAt:   time.Now().UTC().Format(time.RFC3339),
			TraceID:       "trace-1",
			Payload:       mustJSON(t, map[string]any{"city": "Austin", "units": "metric"}),
		}),
	}

	if err := w.processFetchedMessage(context.Background(), msg); err != nil {
		t.Fatalf("processFetchedMessage() error = %v", err)
	}
	if consumer.commitCalls != 1 {
		t.Fatalf("commitCalls = %d, want 1", consumer.commitCalls)
	}
	if !reflect.DeepEqual(events, []string{"handler", "commit"}) {
		t.Fatalf("events = %v, want [handler commit]", events)
	}
}

// TestProcessFetchedMessageSkipsCommitOnHandlerFailure verifies retry semantics.
func TestProcessFetchedMessageSkipsCommitOnHandlerFailure(t *testing.T) {
	t.Parallel()

	consumer := &fakeConsumer{}
	w := &worker{
		cfg: config{
			processTimeout: time.Second,
			commitTimeout:  time.Second,
		},
		logger:      log.New(testWriter{t: t}, "", 0),
		consumer:    consumer,
		statusStore: noOpStatusStore{},
		resultStore: noOpResultStore{},
		handlers: map[string]jobHandler{
			"weather": func(context.Context, kafkaJobMessage) (jobExecutionResult, error) {
				return jobExecutionResult{}, errors.New("processing failed")
			},
		},
	}

	msg := kafka.Message{
		Topic: "jobs.weather.v1",
		Key:   []byte("job-2"),
		Value: mustKafkaMessage(t, kafkaJobMessage{
			SchemaVersion: "1.0",
			JobID:         "job-2",
			JobType:       "weather",
			SubmittedAt:   time.Now().UTC().Format(time.RFC3339),
			TraceID:       "trace-2",
			Payload:       mustJSON(t, map[string]any{"city": "Austin", "units": "metric"}),
		}),
	}

	if err := w.processFetchedMessage(context.Background(), msg); err == nil {
		t.Fatalf("processFetchedMessage() error = nil, want non-nil")
	}
	if consumer.commitCalls != 0 {
		t.Fatalf("commitCalls = %d, want 0", consumer.commitCalls)
	}
}

// TestProcessFetchedMessageCommitsInvalidEnvelope verifies poison-message drop behavior.
func TestProcessFetchedMessageCommitsInvalidEnvelope(t *testing.T) {
	t.Parallel()

	consumer := &fakeConsumer{}
	w := &worker{
		cfg: config{
			commitTimeout: time.Second,
		},
		logger:      log.New(testWriter{t: t}, "", 0),
		consumer:    consumer,
		statusStore: noOpStatusStore{},
		resultStore: noOpResultStore{},
		handlers:    map[string]jobHandler{},
	}

	msg := kafka.Message{
		Topic: "jobs.weather.v1",
		Key:   []byte("job-3"),
		Value: []byte("not-json"),
	}

	if err := w.processFetchedMessage(context.Background(), msg); err != nil {
		t.Fatalf("processFetchedMessage() error = %v, want nil", err)
	}
	if consumer.commitCalls != 1 {
		t.Fatalf("commitCalls = %d, want 1", consumer.commitCalls)
	}
}

// TestDecodeProgressCheckRequest validates required request fields for RabbitMQ status checks.
func TestDecodeProgressCheckRequest(t *testing.T) {
	t.Parallel()

	valid := mustJSON(t, map[string]any{
		"job_id":       "6aab8fca-7059-40c4-97d4-53f55fd5bf67",
		"request_id":   "f2ce7230-d853-4a5f-ab27-bf20a4f5e273",
		"requested_at": "2026-02-15T08:00:00Z",
	})
	if _, err := decodeProgressCheckRequest(valid); err != nil {
		t.Fatalf("decodeProgressCheckRequest(valid) error = %v", err)
	}

	invalid := mustJSON(t, map[string]any{
		"job_id":       "",
		"request_id":   "f2ce7230-d853-4a5f-ab27-bf20a4f5e273",
		"requested_at": "2026-02-15T08:00:00Z",
	})
	if _, err := decodeProgressCheckRequest(invalid); err == nil {
		t.Fatalf("decodeProgressCheckRequest(invalid) error = nil, want non-nil")
	}
}

// TestParseProgressPercent verifies bounds-safe conversion from Redis values.
func TestParseProgressPercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want int
	}{
		{raw: "", want: 0},
		{raw: "-5", want: 0},
		{raw: "25", want: 25},
		{raw: "100", want: 100},
		{raw: "250", want: 100},
		{raw: "not-int", want: 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()
			got := parseProgressPercent(tt.raw)
			if got != tt.want {
				t.Fatalf("parseProgressPercent(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

// TestHandleWeatherJobRetriesTransientFailure verifies transient provider retry behavior.
func TestHandleWeatherJobRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	weather := &fakeWeatherClient{
		results: []weatherFetchResult{
			{err: errors.New("weather provider returned status=503 body=\"temporarily unavailable\"")},
			{obs: weatherObservation{Temperature: 20.4, Condition: "Cloudy", HumidityPct: 61, WindKPH: 13.2, Provider: "mock"}},
		},
	}
	w := &worker{
		cfg: config{
			weatherMaxAttempts:    3,
			weatherRetryInitialBO: 0,
			weatherRetryMaxBO:     0,
		},
		logger:  log.New(testWriter{t: t}, "", 0),
		weather: weather,
	}

	result, err := w.handleWeatherJob(context.Background(), kafkaJobMessage{
		JobID:   "job-weather-retry",
		TraceID: "trace-weather-retry",
		Payload: mustJSON(t, map[string]any{
			"city":         "Austin",
			"country_code": "US",
			"units":        "metric",
		}),
	})
	if err != nil {
		t.Fatalf("handleWeatherJob() error = %v", err)
	}
	if weather.calls != 2 {
		t.Fatalf("weather calls = %d, want 2", weather.calls)
	}
	if got := result.Output["provider"]; got != "mock" {
		t.Fatalf("result provider = %v, want mock", got)
	}
}

// TestHandleWeatherJobSkipsRetryForNonRetryableError verifies coordinate failures do not retry.
func TestHandleWeatherJobSkipsRetryForNonRetryableError(t *testing.T) {
	t.Parallel()

	weather := &fakeWeatherClient{
		results: []weatherFetchResult{
			{err: errors.New("no coordinates found for city=Unknown country_code=")},
		},
	}
	w := &worker{
		cfg: config{
			weatherMaxAttempts:    3,
			weatherRetryInitialBO: 0,
			weatherRetryMaxBO:     0,
		},
		logger:  log.New(testWriter{t: t}, "", 0),
		weather: weather,
	}

	_, err := w.handleWeatherJob(context.Background(), kafkaJobMessage{
		JobID:   "job-weather-no-retry",
		TraceID: "trace-weather-no-retry",
		Payload: mustJSON(t, map[string]any{
			"city":  "Unknown",
			"units": "metric",
		}),
	})
	if err == nil {
		t.Fatalf("handleWeatherJob() error = nil, want non-nil")
	}
	if weather.calls != 1 {
		t.Fatalf("weather calls = %d, want 1", weather.calls)
	}
}

// TestCalculateRetryBackoff verifies exponential backoff with max capping.
func TestCalculateRetryBackoff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		initial time.Duration
		max     time.Duration
		attempt int
		want    time.Duration
	}{
		{name: "attempt one", initial: 500 * time.Millisecond, max: 4 * time.Second, attempt: 1, want: 500 * time.Millisecond},
		{name: "attempt three", initial: 500 * time.Millisecond, max: 4 * time.Second, attempt: 3, want: 2 * time.Second},
		{name: "max cap", initial: 500 * time.Millisecond, max: 1500 * time.Millisecond, attempt: 3, want: 1500 * time.Millisecond},
		{name: "no initial", initial: 0, max: 4 * time.Second, attempt: 3, want: 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := calculateRetryBackoff(tt.initial, tt.max, tt.attempt)
			if got != tt.want {
				t.Fatalf("calculateRetryBackoff() = %s, want %s", got, tt.want)
			}
		})
	}
}

// testWriter routes logger output into test logs.
type testWriter struct {
	t *testing.T
}

// Write sends log bytes into t.Log for deterministic test output capture.
func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(string(p))
	return len(p), nil
}

// mustKafkaMessage marshals job envelopes for test fixtures.
func mustKafkaMessage(t *testing.T, msg kafkaJobMessage) []byte {
	t.Helper()
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}
	return b
}

// mustJSON marshals arbitrary payload fixtures.
func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}
	return b
}
