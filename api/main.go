package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

// appLogger is the process-wide logger used for startup and helper-function logs.
var appLogger = log.New(os.Stdout, "api ", log.LstdFlags|log.LUTC)

// config contains runtime settings loaded from environment variables.
type config struct {
	listenAddr         string
	kafkaBrokers       []string
	kafkaTopic         string
	kafkaTopicTemplate string
	redisAddr          string
	redisUsername      string
	redisPassword      string
	redisDB            int
	statusTTL          time.Duration
	requestTimeout     time.Duration
}

// app bundles service dependencies and configuration.
type app struct {
	cfg         config
	logger      *log.Logger
	kafkaWriter *kafka.Writer
	redisClient *redis.Client
}

// submitJobRequest is the generic public job-submission request.
type submitJobRequest struct {
	JobType string          `json:"job_type"`
	Payload json.RawMessage `json:"payload"`
}

// submitWeatherRequest is the weather-specific payload shape.
type submitWeatherRequest struct {
	City        string `json:"city"`
	CountryCode string `json:"country_code,omitempty"`
	Units       string `json:"units"`
}

// submitJobResponse is returned when a job is accepted for processing.
type submitJobResponse struct {
	JobID       string `json:"job_id"`
	TraceID     string `json:"trace_id"`
	JobType     string `json:"job_type"`
	State       string `json:"state"`
	SubmittedAt string `json:"submitted_at"`
	Message     string `json:"message"`
}

// errorResponse is the uniform JSON error envelope.
type errorResponse struct {
	Error string `json:"error"`
}

// kafkaJobMessage is the canonical Kafka submission envelope.
type kafkaJobMessage struct {
	SchemaVersion string          `json:"schema_version"`
	JobID         string          `json:"job_id"`
	JobType       string          `json:"job_type"`
	SubmittedAt   string          `json:"submitted_at"`
	TraceID       string          `json:"trace_id"`
	Payload       json.RawMessage `json:"payload"`
}

// payloadValidator validates and normalizes a job payload.
type payloadValidator func(json.RawMessage) (json.RawMessage, error)

// jobPayloadValidators defines currently supported job types and their validators.
var jobPayloadValidators = map[string]payloadValidator{
	"weather":       validateWeatherPayload,
	"quote":         validateObjectPayload,
	"exchange_rate": validateObjectPayload,
	"github_user":   validateObjectPayload,
}

// main boots the API process and manages graceful shutdown.
func main() {
	cfg, err := loadConfig()
	if err != nil {
		appLogger.Fatalf("config load failed: %v", err)
	}

	a := newApp(cfg, appLogger)
	defer a.close()

	srv := &http.Server{
		Addr:              cfg.listenAddr,
		Handler:           a.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-shutdownCtx.Done()
		a.logger.Println("shutdown signal received")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			a.logger.Printf("graceful shutdown failed: %v", err)
		}
	}()

	a.logger.Printf("starting API on %s", cfg.listenAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		a.logger.Fatalf("server failed: %v", err)
	}
	a.logger.Println("server stopped")
}

// loadConfig parses all environment-driven runtime settings.
func loadConfig() (config, error) {
	redisDB, err := parseIntEnv("REDIS_DB", 0)
	if err != nil {
		return config{}, err
	}

	statusTTL, err := parseDurationEnv("STATUS_TTL", 24*time.Hour)
	if err != nil {
		return config{}, err
	}

	requestTimeout, err := parseDurationEnv("REQUEST_TIMEOUT", 5*time.Second)
	if err != nil {
		return config{}, err
	}

	cfg := config{
		listenAddr:         envOrDefault("API_ADDR", ":8080"),
		kafkaBrokers:       parseCSVEnv("KAFKA_BROKERS", []string{"localhost:9094"}),
		kafkaTopic:         strings.TrimSpace(os.Getenv("KAFKA_TOPIC")),
		kafkaTopicTemplate: envOrDefault("KAFKA_TOPIC_TEMPLATE", "jobs.%s.v1"),
		redisAddr:          envOrDefault("REDIS_ADDR", "localhost:6379"),
		redisUsername:      os.Getenv("REDIS_USERNAME"),
		redisPassword:      os.Getenv("REDIS_PASSWORD"),
		redisDB:            redisDB,
		statusTTL:          statusTTL,
		requestTimeout:     requestTimeout,
	}

	appLogger.Printf(
		"config loaded addr=%s kafka_brokers=%v kafka_topic=%q kafka_topic_template=%q redis_addr=%s redis_db=%d status_ttl=%s request_timeout=%s",
		cfg.listenAddr,
		cfg.kafkaBrokers,
		cfg.kafkaTopic,
		cfg.kafkaTopicTemplate,
		cfg.redisAddr,
		cfg.redisDB,
		cfg.statusTTL,
		cfg.requestTimeout,
	)
	return cfg, nil
}

// newApp initializes all dependency clients.
func newApp(cfg config, logger *log.Logger) *app {
	logger.Println("initializing application dependencies")
	return &app{
		cfg:    cfg,
		logger: logger,
		kafkaWriter: &kafka.Writer{
			Addr:                   kafka.TCP(cfg.kafkaBrokers...),
			Balancer:               &kafka.LeastBytes{},
			RequiredAcks:           kafka.RequireAll,
			AllowAutoTopicCreation: true,
			Async:                  false,
			WriteTimeout:           cfg.requestTimeout,
			ReadTimeout:            cfg.requestTimeout,
		},
		redisClient: redis.NewClient(&redis.Options{
			Addr:     cfg.redisAddr,
			Username: cfg.redisUsername,
			Password: cfg.redisPassword,
			DB:       cfg.redisDB,
		}),
	}
}

// close closes network clients during shutdown.
func (a *app) close() {
	a.logger.Println("closing dependencies")
	if err := a.kafkaWriter.Close(); err != nil {
		a.logger.Printf("kafka writer close failed: %v", err)
	}
	if err := a.redisClient.Close(); err != nil {
		a.logger.Printf("redis close failed: %v", err)
	}
}

// routes registers HTTP endpoints.
func (a *app) routes() http.Handler {
	a.logger.Println("registering routes: GET /healthz, POST /v1/jobs")
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", a.handleHealthz)
	mux.HandleFunc("/v1/jobs", a.handleSubmitJob)
	return mux
}

// handleHealthz reports API dependency health (Kafka + Redis).
func (a *app) handleHealthz(w http.ResponseWriter, r *http.Request) {
	a.logger.Printf("healthz request method=%s", r.Method)
	if r.Method != http.MethodGet {
		a.logger.Printf("healthz rejected method=%s", r.Method)
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), a.cfg.requestTimeout)
	defer cancel()

	redisOK := true
	if err := a.redisClient.Ping(ctx).Err(); err != nil {
		redisOK = false
		a.logger.Printf("healthz redis ping failed: %v", err)
	}

	kafkaOK := false
	for _, broker := range a.cfg.kafkaBrokers {
		conn, err := kafka.DialContext(ctx, "tcp", broker)
		if err != nil {
			a.logger.Printf("healthz kafka dial failed broker=%s err=%v", broker, err)
			continue
		}
		kafkaOK = true
		_ = conn.Close()
		break
	}

	statusCode := http.StatusOK
	overall := "ok"
	if !kafkaOK || !redisOK {
		statusCode = http.StatusServiceUnavailable
		overall = "degraded"
	}

	a.logger.Printf("healthz result status=%s kafka_ok=%t redis_ok=%t", overall, kafkaOK, redisOK)
	writeJSON(w, statusCode, map[string]any{
		"status": overall,
		"checks": map[string]bool{
			"kafka": kafkaOK,
			"redis": redisOK,
		},
		"time": time.Now().UTC().Format(time.RFC3339),
	})
}

// handleSubmitJob accepts generic job submissions.
func (a *app) handleSubmitJob(w http.ResponseWriter, r *http.Request) {
	a.logger.Printf("submit job request method=%s path=%s", r.Method, r.URL.Path)
	if r.Method != http.MethodPost {
		a.logger.Printf("submit job rejected method=%s", r.Method)
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	var req submitJobRequest
	if err := decodeJSON(r, &req); err != nil {
		a.logger.Printf("submit job invalid request: %v", err)
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	req, err := validateSubmitJobRequest(req)
	if err != nil {
		a.logger.Printf("submit job validation failed: %v", err)
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	jobID, traceID, submittedAt, err := a.enqueueJob(r.Context(), req.JobType, req.Payload)
	if err != nil {
		a.logger.Printf("submit job enqueue failed job_type=%s err=%v", req.JobType, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/v1/jobs/%s/status", jobID))
	writeJSON(w, http.StatusAccepted, submitJobResponse{
		JobID:       jobID,
		TraceID:     traceID,
		JobType:     req.JobType,
		State:       "queued",
		SubmittedAt: submittedAt.Format(time.RFC3339),
		Message:     "job accepted",
	})
	a.logger.Printf("submit job accepted job_id=%s trace_id=%s job_type=%s", jobID, traceID, req.JobType)
}

// enqueueJob writes initial Redis status and publishes the canonical Kafka message.
func (a *app) enqueueJob(parentCtx context.Context, jobType string, payload json.RawMessage) (string, string, time.Time, error) {
	jobID := uuid.NewString()
	traceID := uuid.NewString()
	now := time.Now().UTC()

	a.logger.Printf("enqueue start job_id=%s trace_id=%s job_type=%s", jobID, traceID, jobType)
	ctx, cancel := context.WithTimeout(parentCtx, a.cfg.requestTimeout)
	defer cancel()

	if err := a.writeQueuedStatus(ctx, jobID, traceID, now); err != nil {
		a.logger.Printf("enqueue redis queued status write failed job_id=%s: %v", jobID, err)
		return "", "", time.Time{}, errors.New("failed to initialize job status")
	}

	jobMsg := kafkaJobMessage{
		SchemaVersion: "1.0",
		JobID:         jobID,
		JobType:       jobType,
		SubmittedAt:   now.Format(time.RFC3339),
		TraceID:       traceID,
		Payload:       payload,
	}

	body, err := json.Marshal(jobMsg)
	if err != nil {
		a.logger.Printf("enqueue kafka payload marshal failed job_id=%s: %v", jobID, err)
		return "", "", time.Time{}, errors.New("failed to encode job")
	}

	topic := a.topicForJobType(jobType)
	kmsg := kafka.Message{
		Topic: topic,
		Key:   []byte(jobID),
		Value: body,
		Headers: []kafka.Header{
			{Key: "content-type", Value: []byte("application/json")},
			{Key: "schema-version", Value: []byte("1.0")},
			{Key: "job-type", Value: []byte(jobType)},
		},
	}

	if err := a.kafkaWriter.WriteMessages(ctx, kmsg); err != nil {
		a.logger.Printf("enqueue kafka publish failed job_id=%s job_type=%s topic=%s err=%v", jobID, jobType, topic, err)
		_ = a.writeFailedEnqueueStatus(context.Background(), jobID, traceID)
		return "", "", time.Time{}, errors.New("failed to enqueue job")
	}

	a.logger.Printf("enqueue success job_id=%s trace_id=%s job_type=%s topic=%s", jobID, traceID, jobType, topic)
	return jobID, traceID, now, nil
}

// topicForJobType resolves Kafka topic from static override or template.
func (a *app) topicForJobType(jobType string) string {
	if a.cfg.kafkaTopic != "" {
		a.logger.Printf("topic resolved from KAFKA_TOPIC override topic=%s", a.cfg.kafkaTopic)
		return a.cfg.kafkaTopic
	}
	if strings.Contains(a.cfg.kafkaTopicTemplate, "%s") {
		topic := fmt.Sprintf(a.cfg.kafkaTopicTemplate, jobType)
		a.logger.Printf("topic resolved from template job_type=%s topic=%s", jobType, topic)
		return topic
	}
	a.logger.Printf("topic resolved from raw template value topic=%s", a.cfg.kafkaTopicTemplate)
	return a.cfg.kafkaTopicTemplate
}

// writeQueuedStatus initializes Redis status for a newly queued job.
func (a *app) writeQueuedStatus(ctx context.Context, jobID, traceID string, at time.Time) error {
	key := jobStatusKey(jobID)
	values := map[string]any{
		"job_id":           jobID,
		"trace_id":         traceID,
		"state":            "queued",
		"progress_percent": 0,
		"updated_at":       at.Format(time.RFC3339),
		"message":          "job queued",
	}

	if err := a.redisClient.HSet(ctx, key, values).Err(); err != nil {
		a.logger.Printf("redis HSET failed key=%s err=%v", key, err)
		return err
	}
	if err := a.redisClient.Expire(ctx, key, a.cfg.statusTTL).Err(); err != nil {
		a.logger.Printf("redis EXPIRE failed key=%s ttl=%s err=%v", key, a.cfg.statusTTL, err)
		return err
	}

	a.logger.Printf("redis queued status written key=%s", key)
	return nil
}

// writeFailedEnqueueStatus marks a job as failed when Kafka publish fails.
func (a *app) writeFailedEnqueueStatus(ctx context.Context, jobID, traceID string) error {
	ctx, cancel := context.WithTimeout(ctx, a.cfg.requestTimeout)
	defer cancel()

	key := jobStatusKey(jobID)
	values := map[string]any{
		"job_id":           jobID,
		"trace_id":         traceID,
		"state":            "failed",
		"progress_percent": 0,
		"updated_at":       time.Now().UTC().Format(time.RFC3339),
		"message":          "failed to enqueue job",
		"error_code":       "KAFKA_PUBLISH_FAILED",
	}

	if err := a.redisClient.HSet(ctx, key, values).Err(); err != nil {
		a.logger.Printf("redis failed-status HSET failed key=%s err=%v", key, err)
		return err
	}
	if err := a.redisClient.Expire(ctx, key, a.cfg.statusTTL).Err(); err != nil {
		a.logger.Printf("redis failed-status EXPIRE failed key=%s ttl=%s err=%v", key, a.cfg.statusTTL, err)
		return err
	}

	a.logger.Printf("redis failed enqueue status written key=%s", key)
	return nil
}

// jobStatusKey returns the canonical Redis key for transient job status.
func jobStatusKey(jobID string) string {
	return fmt.Sprintf("job:%s:status", jobID)
}

// decodeJSON decodes a strict single-object JSON request body with size limit.
func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		appLogger.Printf("decodeJSON failed: %v", err)
		return fmt.Errorf("invalid request body: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		appLogger.Printf("decodeJSON multiple values detected")
		return errors.New("invalid request body: multiple JSON values")
	}
	return nil
}

// validateSubmitJobRequest validates and normalizes the generic submit request.
func validateSubmitJobRequest(in submitJobRequest) (submitJobRequest, error) {
	in.JobType = strings.ToLower(strings.TrimSpace(in.JobType))
	if in.JobType == "" {
		appLogger.Printf("validate submit job failed: missing job_type")
		return in, errors.New("job_type is required")
	}
	if len(in.Payload) == 0 {
		appLogger.Printf("validate submit job failed: missing payload")
		return in, errors.New("payload is required")
	}

	validator, ok := jobPayloadValidators[in.JobType]
	if !ok {
		err := fmt.Errorf("unsupported job_type: %s (supported: %s)", in.JobType, strings.Join(supportedJobTypes(), ", "))
		appLogger.Printf("validate submit job failed: %v", err)
		return in, err
	}

	normalizedPayload, err := validator(in.Payload)
	if err != nil {
		appLogger.Printf("validate submit job failed for job_type=%s: %v", in.JobType, err)
		return in, err
	}
	in.Payload = normalizedPayload
	return in, nil
}

// validateWeatherPayload validates weather payload fields and normalizes casing.
func validateWeatherPayload(raw json.RawMessage) (json.RawMessage, error) {
	var in submitWeatherRequest
	dec := json.NewDecoder(bytesReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		appLogger.Printf("validate weather payload decode failed: %v", err)
		return nil, fmt.Errorf("invalid weather payload: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		appLogger.Printf("validate weather payload failed: multiple JSON values")
		return nil, errors.New("invalid weather payload: multiple JSON values")
	}

	in.City = strings.TrimSpace(in.City)
	in.CountryCode = strings.ToUpper(strings.TrimSpace(in.CountryCode))
	in.Units = strings.ToLower(strings.TrimSpace(in.Units))

	if in.City == "" {
		appLogger.Printf("validate weather payload failed: city is required")
		return nil, errors.New("weather payload city is required")
	}
	if in.Units != "metric" && in.Units != "imperial" {
		appLogger.Printf("validate weather payload failed: invalid units=%s", in.Units)
		return nil, errors.New("weather payload units must be one of: metric, imperial")
	}
	if in.CountryCode != "" && len(in.CountryCode) != 2 {
		appLogger.Printf("validate weather payload failed: invalid country_code=%s", in.CountryCode)
		return nil, errors.New("weather payload country_code must be ISO 3166-1 alpha-2")
	}

	out, err := json.Marshal(in)
	if err != nil {
		appLogger.Printf("validate weather payload marshal failed: %v", err)
		return nil, errors.New("failed to normalize weather payload")
	}
	return out, nil
}

// validateObjectPayload verifies payload is a JSON object and normalizes encoding.
func validateObjectPayload(raw json.RawMessage) (json.RawMessage, error) {
	var payload map[string]any
	dec := json.NewDecoder(bytesReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		appLogger.Printf("validate object payload decode failed: %v", err)
		return nil, fmt.Errorf("invalid payload: expected JSON object: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		appLogger.Printf("validate object payload failed: multiple JSON values")
		return nil, errors.New("invalid payload: multiple JSON values")
	}
	if payload == nil {
		appLogger.Printf("validate object payload failed: nil object")
		return nil, errors.New("invalid payload: expected JSON object")
	}

	out, err := json.Marshal(payload)
	if err != nil {
		appLogger.Printf("validate object payload marshal failed: %v", err)
		return nil, errors.New("failed to normalize payload")
	}
	return out, nil
}

// bytesReader creates a reusable io.Reader from byte slices.
func bytesReader(b []byte) io.Reader {
	return bytes.NewReader(b)
}

// supportedJobTypes returns sorted currently supported job-type keys.
func supportedJobTypes() []string {
	keys := make([]string, 0, len(jobPayloadValidators))
	for key := range jobPayloadValidators {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// writeJSON writes a JSON response payload with status code.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		appLogger.Printf("writeJSON encode failed status=%d err=%v", status, err)
		return
	}
	if status >= 400 {
		appLogger.Printf("response sent status=%d", status)
	}
}

// envOrDefault returns an env var value or fallback when unset/blank.
func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		appLogger.Printf("env %s not set; using default=%q", key, fallback)
		return fallback
	}
	return value
}

// parseCSVEnv parses comma-separated env vars into a trimmed slice.
func parseCSVEnv(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		appLogger.Printf("env %s not set; using default list=%v", key, fallback)
		return fallback
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		appLogger.Printf("env %s parsed empty list; using fallback=%v", key, fallback)
		return fallback
	}
	return out
}

// parseIntEnv parses integer env values with fallback.
func parseIntEnv(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		appLogger.Printf("env %s not set; using default int=%d", key, fallback)
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		appLogger.Printf("env %s invalid int value=%q err=%v", key, raw, err)
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return value, nil
}

// parseDurationEnv parses duration env values with fallback.
func parseDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		appLogger.Printf("env %s not set; using default duration=%s", key, fallback)
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		appLogger.Printf("env %s invalid duration value=%q err=%v", key, raw, err)
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}
	return value, nil
}
