package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// appLogger is the process-wide logger used across startup and runtime paths.
var appLogger = log.New(os.Stdout, "worker ", log.LstdFlags|log.LUTC)

// config contains all runtime options for the worker process.
type config struct {
	kafkaBrokers    []string
	kafkaGroupID    string
	kafkaTopic      string
	kafkaTopicTpl   string
	kafkaTopics     []string
	jobTypes        []string
	fetchMinBytes   int
	fetchMaxBytes   int
	fetchMaxWait    time.Duration
	fetchErrBackoff time.Duration
	processTimeout  time.Duration
	commitTimeout   time.Duration

	redisAddr     string
	redisUsername string
	redisPassword string
	redisDB       int
	statusTTL     time.Duration

	mongoURI        string
	mongoDatabase   string
	mongoCollection string
	mongoConnTO     time.Duration

	rabbitURL                string
	progressRequestQueue     string
	progressConsumerTag      string
	progressConsumerPrefetch int
	progressReconnectBackoff time.Duration

	weatherProvider        string
	weatherHTTPTimeout     time.Duration
	weatherGeocodingURL    string
	weatherForecastURL     string
	weatherUseMockFallback bool
	weatherMaxAttempts     int
	weatherRetryInitialBO  time.Duration
	weatherRetryMaxBO      time.Duration
}

// kafkaConsumer abstracts kafka.Reader for testability of commit behavior.
type kafkaConsumer interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// statusStore writes transient job status updates.
type statusStore interface {
	UpsertStatus(ctx context.Context, status jobStatusRecord) error
}

// resultStore writes durable final job results.
type resultStore interface {
	UpsertJobResult(ctx context.Context, doc jobResultDocument) (bool, error)
}

// weatherClient fetches normalized weather observations.
type weatherClient interface {
	Fetch(ctx context.Context, payload weatherPayload) (weatherObservation, error)
}

// worker wires the consumer loop with typed job handlers and stores.
type worker struct {
	cfg         config
	logger      *log.Logger
	consumer    kafkaConsumer
	redisClient *redis.Client
	mongoClient *mongo.Client
	rabbitConn  *amqp.Connection
	rabbitChan  *amqp.Channel
	statusStore statusStore
	resultStore resultStore
	weather     weatherClient
	handlers    map[string]jobHandler
}

// jobHandler executes job-type-specific processing.
type jobHandler func(ctx context.Context, job kafkaJobMessage) (jobExecutionResult, error)

// jobExecutionResult carries normalized input and output for persistence.
type jobExecutionResult struct {
	Input   map[string]any
	Output  map[string]any
	Message string
}

// kafkaJobMessage is the canonical Week 1 Kafka job envelope.
type kafkaJobMessage struct {
	SchemaVersion string          `json:"schema_version"`
	JobID         string          `json:"job_id"`
	JobType       string          `json:"job_type"`
	SubmittedAt   string          `json:"submitted_at"`
	TraceID       string          `json:"trace_id"`
	Payload       json.RawMessage `json:"payload"`
}

// progressCheckRequest models RabbitMQ status request messages.
type progressCheckRequest struct {
	JobID       string `json:"job_id"`
	RequestID   string `json:"request_id"`
	RequestedAt string `json:"requested_at"`
}

// progressCheckReply models RabbitMQ status reply messages.
type progressCheckReply struct {
	JobID           string `json:"job_id"`
	State           string `json:"state"`
	ProgressPercent int    `json:"progress_percent"`
	Message         string `json:"message"`
	Timestamp       string `json:"timestamp"`
}

// weatherPayload is the weather profile payload shape used first in Week 1.
type weatherPayload struct {
	City        string `json:"city"`
	CountryCode string `json:"country_code,omitempty"`
	Units       string `json:"units"`
}

// weatherObservation is the normalized weather output persisted in MongoDB.
type weatherObservation struct {
	Temperature float64
	Condition   string
	HumidityPct int
	WindKPH     float64
	Provider    string
}

// jobStatusRecord models one Redis status update.
type jobStatusRecord struct {
	JobID           string
	TraceID         string
	State           string
	ProgressPercent int
	Message         string
	UpdatedAt       time.Time
	ErrorCode       string
	ErrorMessage    string
}

// jobResultDocument models MongoDB job_results documents.
type jobResultDocument struct {
	SchemaVersion string          `bson:"schema_version"`
	JobID         string          `bson:"job_id"`
	JobType       string          `bson:"job_type"`
	Input         map[string]any  `bson:"input"`
	Output        map[string]any  `bson:"output"`
	FinalState    string          `bson:"final_state"`
	StartedAt     string          `bson:"started_at"`
	CompletedAt   string          `bson:"completed_at"`
	Error         *jobErrorRecord `bson:"error"`
	TraceID       string          `bson:"trace_id"`
}

// jobErrorRecord models optional terminal job errors for Mongo documents.
type jobErrorRecord struct {
	Code    string `bson:"code"`
	Message string `bson:"message"`
}

// redisHashStatusStore persists status records into the canonical Redis hash key.
type redisHashStatusStore struct {
	client *redis.Client
	ttl    time.Duration
	logger *log.Logger
}

// mongoJobResultStore persists final results into MongoDB with idempotent upsert.
type mongoJobResultStore struct {
	collection *mongo.Collection
	logger     *log.Logger
}

// openMeteoWeatherClient calls Open-Meteo geocoding + current weather APIs.
type openMeteoWeatherClient struct {
	httpClient   *http.Client
	geocodingURL string
	forecastURL  string
	logger       *log.Logger
}

// fallbackWeatherClient falls back to mock results when primary provider fails.
type fallbackWeatherClient struct {
	primary  weatherClient
	fallback weatherClient
	logger   *log.Logger
}

// mockWeatherClient returns deterministic observations for offline local development.
type mockWeatherClient struct{}

// supportedJobTypes defines job types this worker can route.
var supportedJobTypes = []string{"weather", "quote", "exchange_rate", "github_user"}

// main boots the worker and handles graceful shutdown signals.
func main() {
	cfg, err := loadConfig()
	if err != nil {
		appLogger.Fatalf("config load failed: %v", err)
	}

	w, err := newWorker(cfg, appLogger)
	if err != nil {
		appLogger.Fatalf("worker init failed: %v", err)
	}
	defer w.close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := w.run(ctx); err != nil {
		appLogger.Fatalf("worker runtime failed: %v", err)
	}
	appLogger.Println("worker stopped cleanly")
}

// loadConfig parses environment variables into a validated runtime config.
func loadConfig() (config, error) {
	fetchMinBytes, err := parseIntEnv("WORKER_FETCH_MIN_BYTES", 1)
	if err != nil {
		return config{}, err
	}

	fetchMaxBytes, err := parseIntEnv("WORKER_FETCH_MAX_BYTES", 10*1024*1024)
	if err != nil {
		return config{}, err
	}

	redisDB, err := parseIntEnv("REDIS_DB", 0)
	if err != nil {
		return config{}, err
	}

	fetchMaxWait, err := parseDurationEnv("WORKER_FETCH_MAX_WAIT", 1*time.Second)
	if err != nil {
		return config{}, err
	}

	fetchErrBackoff, err := parseDurationEnv("WORKER_FETCH_ERROR_BACKOFF", 500*time.Millisecond)
	if err != nil {
		return config{}, err
	}

	processTimeout, err := parseDurationEnv("WORKER_PROCESS_TIMEOUT", 45*time.Second)
	if err != nil {
		return config{}, err
	}

	commitTimeout, err := parseDurationEnv("WORKER_COMMIT_TIMEOUT", 5*time.Second)
	if err != nil {
		return config{}, err
	}

	statusTTL, err := parseDurationEnv("STATUS_TTL", 24*time.Hour)
	if err != nil {
		return config{}, err
	}

	mongoConnTimeout, err := parseDurationEnv("MONGO_CONNECT_TIMEOUT", 10*time.Second)
	if err != nil {
		return config{}, err
	}

	weatherHTTPTimeout, err := parseDurationEnv("WEATHER_HTTP_TIMEOUT", 8*time.Second)
	if err != nil {
		return config{}, err
	}

	progressConsumerPrefetch, err := parseIntEnv("RABBITMQ_PROGRESS_PREFETCH", 20)
	if err != nil {
		return config{}, err
	}

	progressReconnectBackoff, err := parseDurationEnv("RABBITMQ_PROGRESS_RECONNECT_BACKOFF", 2*time.Second)
	if err != nil {
		return config{}, err
	}

	weatherMockFallback, err := parseBoolEnv("WEATHER_USE_MOCK_FALLBACK", true)
	if err != nil {
		return config{}, err
	}

	weatherMaxAttempts, err := parseIntEnv("WEATHER_MAX_ATTEMPTS", 3)
	if err != nil {
		return config{}, err
	}
	if weatherMaxAttempts < 1 {
		return config{}, errors.New("WEATHER_MAX_ATTEMPTS must be >= 1")
	}

	weatherRetryInitialBackoff, err := parseDurationEnv("WEATHER_RETRY_INITIAL_BACKOFF", 500*time.Millisecond)
	if err != nil {
		return config{}, err
	}

	weatherRetryMaxBackoff, err := parseDurationEnv("WEATHER_RETRY_MAX_BACKOFF", 4*time.Second)
	if err != nil {
		return config{}, err
	}

	jobTypes, err := normalizeJobTypes(parseCSVEnv("WORKER_JOB_TYPES", []string{"weather"}))
	if err != nil {
		return config{}, err
	}

	cfg := config{
		kafkaBrokers:    parseCSVEnv("WORKER_KAFKA_BROKERS", []string{"localhost:9094"}),
		kafkaGroupID:    envOrDefault("WORKER_KAFKA_GROUP_ID", "dtq-worker-v1"),
		kafkaTopic:      strings.TrimSpace(os.Getenv("WORKER_KAFKA_TOPIC")),
		kafkaTopicTpl:   envOrDefault("WORKER_KAFKA_TOPIC_TEMPLATE", "jobs.%s.v1"),
		jobTypes:        jobTypes,
		fetchMinBytes:   fetchMinBytes,
		fetchMaxBytes:   fetchMaxBytes,
		fetchMaxWait:    fetchMaxWait,
		fetchErrBackoff: fetchErrBackoff,
		processTimeout:  processTimeout,
		commitTimeout:   commitTimeout,

		redisAddr:     envOrDefault("REDIS_ADDR", "localhost:6379"),
		redisUsername: strings.TrimSpace(os.Getenv("REDIS_USERNAME")),
		redisPassword: strings.TrimSpace(os.Getenv("REDIS_PASSWORD")),
		redisDB:       redisDB,
		statusTTL:     statusTTL,

		mongoURI:        envOrDefault("MONGO_URI", "mongodb://localhost:27017"),
		mongoDatabase:   envOrDefault("MONGO_DB", "dtq"),
		mongoCollection: envOrDefault("MONGO_COLLECTION", "job_results"),
		mongoConnTO:     mongoConnTimeout,

		rabbitURL:                envOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		progressRequestQueue:     envOrDefault("RABBITMQ_PROGRESS_REQUEST_QUEUE", "progress.check.request.v1"),
		progressConsumerTag:      envOrDefault("RABBITMQ_PROGRESS_CONSUMER_TAG", "dtq-worker-progress-v1"),
		progressConsumerPrefetch: progressConsumerPrefetch,
		progressReconnectBackoff: progressReconnectBackoff,

		weatherProvider:        strings.ToLower(envOrDefault("WEATHER_PROVIDER", "openmeteo")),
		weatherHTTPTimeout:     weatherHTTPTimeout,
		weatherGeocodingURL:    envOrDefault("WEATHER_GEOCODING_URL", "https://geocoding-api.open-meteo.com/v1/search"),
		weatherForecastURL:     envOrDefault("WEATHER_FORECAST_URL", "https://api.open-meteo.com/v1/forecast"),
		weatherUseMockFallback: weatherMockFallback,
		weatherMaxAttempts:     weatherMaxAttempts,
		weatherRetryInitialBO:  weatherRetryInitialBackoff,
		weatherRetryMaxBO:      weatherRetryMaxBackoff,
	}

	cfg.kafkaTopics = resolveKafkaTopics(cfg.jobTypes, cfg.kafkaTopic, cfg.kafkaTopicTpl)
	appLogger.Printf(
		"config loaded kafka_brokers=%v kafka_group_id=%s job_types=%v kafka_topics=%v redis_addr=%s redis_db=%d mongo_uri=%s mongo_db=%s mongo_collection=%s rabbit_progress_queue=%s weather_provider=%s weather_max_attempts=%d",
		cfg.kafkaBrokers,
		cfg.kafkaGroupID,
		cfg.jobTypes,
		cfg.kafkaTopics,
		cfg.redisAddr,
		cfg.redisDB,
		cfg.mongoURI,
		cfg.mongoDatabase,
		cfg.mongoCollection,
		cfg.progressRequestQueue,
		cfg.weatherProvider,
		cfg.weatherMaxAttempts,
	)
	return cfg, nil
}

// newWorker builds dependency clients, stores, and handler registry.
func newWorker(cfg config, logger *log.Logger) (*worker, error) {
	if len(cfg.kafkaTopics) == 0 {
		return nil, errors.New("at least one kafka topic must be configured")
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.kafkaBrokers,
		GroupID:     cfg.kafkaGroupID,
		GroupTopics: cfg.kafkaTopics,
		MinBytes:    cfg.fetchMinBytes,
		MaxBytes:    cfg.fetchMaxBytes,
		MaxWait:     cfg.fetchMaxWait,
	})

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.redisAddr,
		Username: cfg.redisUsername,
		Password: cfg.redisPassword,
		DB:       cfg.redisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), cfg.mongoConnTO)
	defer cancel()

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.mongoURI))
	if err != nil {
		_ = reader.Close()
		_ = redisClient.Close()
		return nil, fmt.Errorf("mongo connect failed: %w", err)
	}

	if err := redisClient.Ping(ctx).Err(); err != nil {
		_ = reader.Close()
		_ = redisClient.Close()
		_ = mongoClient.Disconnect(context.Background())
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	if err := mongoClient.Ping(ctx, nil); err != nil {
		_ = reader.Close()
		_ = redisClient.Close()
		_ = mongoClient.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo ping failed: %w", err)
	}

	collection := mongoClient.Database(cfg.mongoDatabase).Collection(cfg.mongoCollection)
	resultStore := &mongoJobResultStore{collection: collection, logger: logger}
	if err := resultStore.ensureIndexes(ctx); err != nil {
		_ = reader.Close()
		_ = redisClient.Close()
		_ = mongoClient.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo index ensure failed: %w", err)
	}

	rabbitConn, err := amqp.Dial(cfg.rabbitURL)
	if err != nil {
		_ = reader.Close()
		_ = redisClient.Close()
		_ = mongoClient.Disconnect(context.Background())
		return nil, fmt.Errorf("rabbitmq connect failed: %w", err)
	}

	rabbitChan, err := rabbitConn.Channel()
	if err != nil {
		_ = rabbitConn.Close()
		_ = reader.Close()
		_ = redisClient.Close()
		_ = mongoClient.Disconnect(context.Background())
		return nil, fmt.Errorf("rabbitmq channel open failed: %w", err)
	}

	if err := configureProgressChannel(rabbitChan, cfg); err != nil {
		_ = rabbitChan.Close()
		_ = rabbitConn.Close()
		_ = reader.Close()
		_ = redisClient.Close()
		_ = mongoClient.Disconnect(context.Background())
		return nil, err
	}

	weatherClient, err := newWeatherClient(cfg, logger)
	if err != nil {
		_ = rabbitChan.Close()
		_ = rabbitConn.Close()
		_ = reader.Close()
		_ = redisClient.Close()
		_ = mongoClient.Disconnect(context.Background())
		return nil, err
	}

	w := &worker{
		cfg:         cfg,
		logger:      logger,
		consumer:    reader,
		redisClient: redisClient,
		mongoClient: mongoClient,
		rabbitConn:  rabbitConn,
		rabbitChan:  rabbitChan,
		statusStore: &redisHashStatusStore{client: redisClient, ttl: cfg.statusTTL, logger: logger},
		resultStore: resultStore,
		weather:     weatherClient,
	}

	w.handlers = map[string]jobHandler{
		"weather":       w.handleWeatherJob,
		"quote":         w.handleGenericJob,
		"exchange_rate": w.handleGenericJob,
		"github_user":   w.handleGenericJob,
	}

	logger.Printf(
		"worker initialized topics=%v group_id=%s progress_request_queue=%s",
		cfg.kafkaTopics,
		cfg.kafkaGroupID,
		cfg.progressRequestQueue,
	)
	return w, nil
}

// configureProgressChannel applies required QoS and queue declarations for progress requests.
func configureProgressChannel(ch *amqp.Channel, cfg config) error {
	if err := ch.Qos(cfg.progressConsumerPrefetch, 0, false); err != nil {
		return fmt.Errorf("rabbitmq qos setup failed: %w", err)
	}
	if _, err := ch.QueueDeclare(cfg.progressRequestQueue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("rabbitmq request queue declare failed: %w", err)
	}
	return nil
}

// close releases Kafka, Redis, and MongoDB resources during shutdown.
func (w *worker) close() {
	w.logger.Println("closing worker dependencies")
	if w.rabbitChan != nil {
		if err := w.rabbitChan.Close(); err != nil {
			w.logger.Printf("rabbit channel close failed: %v", err)
		}
	}
	if w.rabbitConn != nil {
		if err := w.rabbitConn.Close(); err != nil {
			w.logger.Printf("rabbit connection close failed: %v", err)
		}
	}
	if err := w.consumer.Close(); err != nil {
		w.logger.Printf("kafka consumer close failed: %v", err)
	}
	if w.redisClient != nil {
		if err := w.redisClient.Close(); err != nil {
			w.logger.Printf("redis close failed: %v", err)
		}
	}
	if w.mongoClient != nil {
		if err := w.mongoClient.Disconnect(context.Background()); err != nil {
			w.logger.Printf("mongo disconnect failed: %v", err)
		}
	}
}

// run starts the long-running fetch/process loop until context cancellation.
func (w *worker) run(ctx context.Context) error {
	w.logger.Printf(
		"worker loops starting topics=%v group_id=%s progress_request_queue=%s",
		w.cfg.kafkaTopics,
		w.cfg.kafkaGroupID,
		w.cfg.progressRequestQueue,
	)

	go func() {
		if err := w.runProgressResponder(ctx); err != nil && ctx.Err() == nil {
			w.logger.Printf("progress responder stopped with error: %v", err)
		}
	}()

	return w.runKafkaLoop(ctx)
}

// runKafkaLoop consumes Kafka messages until cancellation and applies job handlers.
func (w *worker) runKafkaLoop(ctx context.Context) error {
	for {
		msg, err := w.consumer.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				w.logger.Println("kafka consumer loop stopping due to cancellation")
				return nil
			}
			w.logger.Printf("kafka fetch failed err=%v; retrying after=%s", err, w.cfg.fetchErrBackoff)
			if err := sleepWithContext(ctx, w.cfg.fetchErrBackoff); err != nil {
				w.logger.Println("kafka fetch backoff canceled")
				return nil
			}
			continue
		}

		if err := w.processFetchedMessage(ctx, msg); err != nil {
			w.logger.Printf(
				"message processing failed topic=%s partition=%d offset=%d key=%s err=%v",
				msg.Topic,
				msg.Partition,
				msg.Offset,
				string(msg.Key),
				err,
			)
		}
	}
}

// runProgressResponder serves RabbitMQ progress check requests until cancellation.
func (w *worker) runProgressResponder(ctx context.Context) error {
	for {
		err := w.runProgressResponderSession(ctx)
		if err == nil || ctx.Err() != nil {
			return nil
		}

		w.logger.Printf(
			"progress responder session failed err=%v; reconnecting after=%s",
			err,
			w.cfg.progressReconnectBackoff,
		)
		if err := sleepWithContext(ctx, w.cfg.progressReconnectBackoff); err != nil {
			w.logger.Println("progress responder reconnect sleep canceled")
			return nil
		}
	}
}

// runProgressResponderSession runs one RabbitMQ consume session until cancel or channel failure.
func (w *worker) runProgressResponderSession(ctx context.Context) error {
	if err := w.reopenProgressChannel(); err != nil {
		return err
	}

	deliveries, err := w.rabbitChan.Consume(
		w.cfg.progressRequestQueue,
		w.cfg.progressConsumerTag,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("rabbitmq consume setup failed: %w", err)
	}

	w.logger.Printf("progress responder consuming queue=%s", w.cfg.progressRequestQueue)
	for {
		select {
		case <-ctx.Done():
			if err := w.rabbitChan.Cancel(w.cfg.progressConsumerTag, false); err != nil {
				w.logger.Printf("progress responder cancel failed: %v", err)
			}
			w.logger.Println("progress responder stopping due to cancellation")
			return nil
		case d, ok := <-deliveries:
			if !ok {
				if ctx.Err() != nil {
					return nil
				}
				return errors.New("rabbitmq deliveries channel closed unexpectedly")
			}

			ack, requeue := w.handleProgressDelivery(ctx, d)
			if ack {
				if err := d.Ack(false); err != nil {
					w.logger.Printf("progress request ack failed delivery_tag=%d err=%v", d.DeliveryTag, err)
				}
				continue
			}

			if err := d.Nack(false, requeue); err != nil {
				w.logger.Printf("progress request nack failed delivery_tag=%d requeue=%t err=%v", d.DeliveryTag, requeue, err)
			}
		}
	}
}

// reopenProgressChannel re-creates the RabbitMQ channel used for request-reply progress handling.
func (w *worker) reopenProgressChannel() error {
	if w.rabbitConn == nil {
		return errors.New("rabbitmq connection is not configured")
	}

	if w.rabbitChan != nil {
		if err := w.rabbitChan.Close(); err != nil {
			w.logger.Printf("progress channel close before reopen failed: %v", err)
		}
	}

	ch, err := w.rabbitConn.Channel()
	if err != nil {
		return fmt.Errorf("rabbitmq channel reopen failed: %w", err)
	}

	if err := configureProgressChannel(ch, w.cfg); err != nil {
		_ = ch.Close()
		return err
	}

	w.rabbitChan = ch
	return nil
}

// handleProgressDelivery validates one status request and publishes the correlated reply.
func (w *worker) handleProgressDelivery(ctx context.Context, d amqp.Delivery) (ack bool, requeue bool) {
	correlationID := strings.TrimSpace(d.CorrelationId)
	replyTo := strings.TrimSpace(d.ReplyTo)
	if correlationID == "" || replyTo == "" {
		w.logger.Printf(
			"dropping invalid progress request missing correlation_id/reply_to delivery_tag=%d correlation_id=%q reply_to=%q",
			d.DeliveryTag,
			correlationID,
			replyTo,
		)
		return true, false
	}

	req, err := decodeProgressCheckRequest(d.Body)
	if err != nil {
		w.logger.Printf("dropping invalid progress request body correlation_id=%s err=%v", correlationID, err)
		return true, false
	}

	if req.RequestID != "" && req.RequestID != correlationID {
		w.logger.Printf(
			"progress request correlation mismatch request_id=%s correlation_id=%s job_id=%s",
			req.RequestID,
			correlationID,
			req.JobID,
		)
	}

	requestCtx, cancel := context.WithTimeout(ctx, w.cfg.commitTimeout)
	defer cancel()

	reply, err := w.buildProgressReply(requestCtx, req.JobID)
	if err != nil {
		w.logger.Printf("progress reply build failed correlation_id=%s job_id=%s err=%v", correlationID, req.JobID, err)
		return false, true
	}

	body, err := json.Marshal(reply)
	if err != nil {
		w.logger.Printf("progress reply marshal failed correlation_id=%s job_id=%s err=%v", correlationID, req.JobID, err)
		return true, false
	}

	if err := w.rabbitChan.PublishWithContext(
		requestCtx,
		"",
		replyTo,
		false,
		false,
		amqp.Publishing{
			ContentType:   "application/json",
			CorrelationId: correlationID,
			ReplyTo:       replyTo,
			Body:          body,
			Timestamp:     time.Now().UTC(),
		},
	); err != nil {
		w.logger.Printf("progress reply publish failed correlation_id=%s job_id=%s err=%v", correlationID, req.JobID, err)
		return false, true
	}

	w.logger.Printf(
		"progress reply published correlation_id=%s job_id=%s state=%s progress=%d",
		correlationID,
		reply.JobID,
		reply.State,
		reply.ProgressPercent,
	)
	return true, false
}

// buildProgressReply loads latest Redis status and projects the canonical reply envelope.
func (w *worker) buildProgressReply(ctx context.Context, jobID string) (progressCheckReply, error) {
	status, found, err := w.readJobStatus(ctx, jobID)
	if err != nil {
		return progressCheckReply{}, err
	}

	if !found {
		return progressCheckReply{
			JobID:           jobID,
			State:           "not_found",
			ProgressPercent: 0,
			Message:         "job not found",
			Timestamp:       time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	state := strings.ToLower(strings.TrimSpace(status["state"]))
	if state == "" {
		state = "not_found"
	}
	progress := parseProgressPercent(status["progress_percent"])
	message := strings.TrimSpace(status["message"])
	if message == "" {
		message = "status available"
	}

	return progressCheckReply{
		JobID:           jobID,
		State:           state,
		ProgressPercent: progress,
		Message:         message,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// readJobStatus fetches the canonical Redis hash for a job status key.
func (w *worker) readJobStatus(ctx context.Context, jobID string) (map[string]string, bool, error) {
	if w.redisClient == nil {
		return nil, false, errors.New("redis client is not configured")
	}
	key := jobStatusKey(jobID)

	values, err := w.redisClient.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, false, err
	}
	if len(values) == 0 {
		return nil, false, nil
	}
	return values, true, nil
}

// processFetchedMessage validates, routes, tracks progress, persists results, and commits offsets.
func (w *worker) processFetchedMessage(ctx context.Context, msg kafka.Message) error {
	job, err := decodeKafkaJobMessage(msg.Value)
	if err != nil {
		w.logger.Printf(
			"dropping invalid message topic=%s partition=%d offset=%d key=%s err=%v",
			msg.Topic,
			msg.Partition,
			msg.Offset,
			string(msg.Key),
			err,
		)
		return w.commitMessage(msg, "drop-invalid-message")
	}

	handler, ok := w.handlers[job.JobType]
	if !ok {
		w.logger.Printf(
			"dropping unsupported job_type job_id=%s trace_id=%s job_type=%s",
			job.JobID,
			job.TraceID,
			job.JobType,
		)
		return w.commitMessage(msg, "drop-unsupported-job-type")
	}

	processCtx, cancel := context.WithTimeout(ctx, w.cfg.processTimeout)
	defer cancel()

	startedAt := time.Now().UTC()
	w.logger.Printf("processing started job_id=%s trace_id=%s job_type=%s", job.JobID, job.TraceID, job.JobType)

	if err := w.updateStatus(processCtx, job, "running", 20, "job started", "", ""); err != nil {
		return fmt.Errorf("status update failed (20%%): %w", err)
	}

	if err := w.updateStatus(processCtx, job, "running", 50, progressMessageForJobType(job.JobType), "", ""); err != nil {
		return fmt.Errorf("status update failed (50%%): %w", err)
	}

	result, err := handler(processCtx, job)
	if err != nil {
		_ = w.updateStatus(context.Background(), job, "failed", 50, "job processing failed", "JOB_PROCESSING_FAILED", err.Error())
		w.logger.Printf(
			"processing failed job_id=%s trace_id=%s job_type=%s err=%v (offset not committed; message can retry)",
			job.JobID,
			job.TraceID,
			job.JobType,
			err,
		)
		return err
	}

	if err := w.updateStatus(processCtx, job, "running", 80, "persisting final result", "", ""); err != nil {
		return fmt.Errorf("status update failed (80%%): %w", err)
	}

	doc := jobResultDocument{
		SchemaVersion: "1.0",
		JobID:         job.JobID,
		JobType:       job.JobType,
		Input:         result.Input,
		Output:        result.Output,
		FinalState:    "completed",
		StartedAt:     startedAt.Format(time.RFC3339),
		CompletedAt:   time.Now().UTC().Format(time.RFC3339),
		Error:         nil,
		TraceID:       job.TraceID,
	}

	inserted, err := w.resultStore.UpsertJobResult(processCtx, doc)
	if err != nil {
		return fmt.Errorf("mongo upsert failed: %w", err)
	}
	if inserted {
		w.logger.Printf("mongo result inserted job_id=%s trace_id=%s job_type=%s", job.JobID, job.TraceID, job.JobType)
	} else {
		w.logger.Printf("mongo result already exists (idempotent replay) job_id=%s trace_id=%s job_type=%s", job.JobID, job.TraceID, job.JobType)
	}

	completionMessage := strings.TrimSpace(result.Message)
	if completionMessage == "" {
		completionMessage = "job completed"
	}
	if err := w.updateStatus(processCtx, job, "completed", 100, completionMessage, "", ""); err != nil {
		return fmt.Errorf("status update failed (100%%): %w", err)
	}

	if err := w.commitMessage(msg, "processed"); err != nil {
		return err
	}

	w.logger.Printf("processing completed job_id=%s trace_id=%s job_type=%s", job.JobID, job.TraceID, job.JobType)
	return nil
}

// updateStatus writes a single Redis status update record for the given job.
func (w *worker) updateStatus(ctx context.Context, job kafkaJobMessage, state string, progress int, message, errorCode, errorMessage string) error {
	if w.statusStore == nil {
		return errors.New("status store is not configured")
	}

	record := jobStatusRecord{
		JobID:           job.JobID,
		TraceID:         job.TraceID,
		State:           state,
		ProgressPercent: progress,
		Message:         message,
		UpdatedAt:       time.Now().UTC(),
		ErrorCode:       errorCode,
		ErrorMessage:    errorMessage,
	}

	if err := w.statusStore.UpsertStatus(ctx, record); err != nil {
		w.logger.Printf("status store write failed job_id=%s trace_id=%s state=%s progress=%d err=%v", job.JobID, job.TraceID, state, progress, err)
		return err
	}

	w.logger.Printf("status updated job_id=%s trace_id=%s state=%s progress=%d", job.JobID, job.TraceID, state, progress)
	return nil
}

// commitMessage records consumer progress only after a message reaches a terminal outcome.
func (w *worker) commitMessage(msg kafka.Message, reason string) error {
	commitCtx, cancel := context.WithTimeout(context.Background(), w.cfg.commitTimeout)
	defer cancel()

	if err := w.consumer.CommitMessages(commitCtx, msg); err != nil {
		w.logger.Printf(
			"offset commit failed reason=%s topic=%s partition=%d offset=%d key=%s err=%v",
			reason,
			msg.Topic,
			msg.Partition,
			msg.Offset,
			string(msg.Key),
			err,
		)
		return err
	}

	w.logger.Printf(
		"offset committed reason=%s topic=%s partition=%d offset=%d key=%s",
		reason,
		msg.Topic,
		msg.Partition,
		msg.Offset,
		string(msg.Key),
	)
	return nil
}

// handleWeatherJob validates weather payload, calls provider, and returns normalized output.
func (w *worker) handleWeatherJob(ctx context.Context, job kafkaJobMessage) (jobExecutionResult, error) {
	var payload weatherPayload
	if err := decodeJSONObject(job.Payload, &payload); err != nil {
		w.logger.Printf("weather payload decode failed job_id=%s trace_id=%s err=%v", job.JobID, job.TraceID, err)
		return jobExecutionResult{}, fmt.Errorf("invalid weather payload: %w", err)
	}

	payload.City = strings.TrimSpace(payload.City)
	payload.CountryCode = strings.ToUpper(strings.TrimSpace(payload.CountryCode))
	payload.Units = strings.ToLower(strings.TrimSpace(payload.Units))

	if payload.City == "" {
		return jobExecutionResult{}, errors.New("weather payload city is required")
	}
	if payload.Units != "metric" && payload.Units != "imperial" {
		return jobExecutionResult{}, errors.New("weather payload units must be one of: metric, imperial")
	}
	if payload.CountryCode != "" && len(payload.CountryCode) != 2 {
		return jobExecutionResult{}, errors.New("weather payload country_code must be ISO 3166-1 alpha-2")
	}

	obs, err := w.fetchWeatherWithRetry(ctx, job, payload)
	if err != nil {
		return jobExecutionResult{}, fmt.Errorf("weather provider request failed: %w", err)
	}

	return jobExecutionResult{
		Input: map[string]any{
			"city":         payload.City,
			"country_code": payload.CountryCode,
			"units":        payload.Units,
		},
		Output: map[string]any{
			"temperature":  obs.Temperature,
			"condition":    obs.Condition,
			"humidity_pct": obs.HumidityPct,
			"wind_kph":     obs.WindKPH,
			"provider":     obs.Provider,
		},
		Message: "weather job completed",
	}, nil
}

// fetchWeatherWithRetry calls the configured weather provider with bounded retry/backoff semantics.
func (w *worker) fetchWeatherWithRetry(ctx context.Context, job kafkaJobMessage, payload weatherPayload) (weatherObservation, error) {
	if w.weather == nil {
		return weatherObservation{}, errors.New("weather client is not configured")
	}

	maxAttempts := w.cfg.weatherMaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		obs, err := w.weather.Fetch(ctx, payload)
		if err == nil {
			if attempt > 1 {
				w.logger.Printf(
					"weather provider recovered job_id=%s trace_id=%s city=%s attempt=%d/%d",
					job.JobID,
					job.TraceID,
					payload.City,
					attempt,
					maxAttempts,
				)
			}
			return obs, nil
		}

		lastErr = err
		if !isRetryableWeatherError(err) {
			w.logger.Printf(
				"weather provider non-retryable failure job_id=%s trace_id=%s city=%s attempt=%d/%d err=%v",
				job.JobID,
				job.TraceID,
				payload.City,
				attempt,
				maxAttempts,
				err,
			)
			return weatherObservation{}, err
		}
		if attempt >= maxAttempts {
			break
		}

		backoff := calculateRetryBackoff(w.cfg.weatherRetryInitialBO, w.cfg.weatherRetryMaxBO, attempt)
		w.logger.Printf(
			"weather provider transient failure job_id=%s trace_id=%s city=%s attempt=%d/%d retry_in=%s err=%v",
			job.JobID,
			job.TraceID,
			payload.City,
			attempt,
			maxAttempts,
			backoff,
			err,
		)

		if err := sleepWithContext(ctx, backoff); err != nil {
			return weatherObservation{}, err
		}
	}

	if lastErr == nil {
		lastErr = errors.New("weather provider failed with unknown error")
	}
	w.logger.Printf(
		"weather provider exhausted retries job_id=%s trace_id=%s city=%s attempts=%d err=%v",
		job.JobID,
		job.TraceID,
		payload.City,
		maxAttempts,
		lastErr,
	)
	return weatherObservation{}, lastErr
}

// isRetryableWeatherError classifies provider errors that should trigger retry/backoff.
func isRetryableWeatherError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, "no coordinates found") {
		return false
	}
	return true
}

// calculateRetryBackoff computes exponential retry delay with optional max cap.
func calculateRetryBackoff(initial, max time.Duration, attempt int) time.Duration {
	if initial <= 0 {
		return 0
	}
	if attempt < 1 {
		attempt = 1
	}

	delay := initial
	for i := 1; i < attempt; i++ {
		if delay > time.Duration(math.MaxInt64/2) {
			delay = time.Duration(math.MaxInt64)
			break
		}
		delay *= 2
	}

	if max > 0 && delay > max {
		return max
	}
	return delay
}

// sleepWithContext waits for duration or returns earlier when context is canceled.
func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// handleGenericJob keeps the worker extensible for non-weather job types in Week 1.
func (w *worker) handleGenericJob(_ context.Context, job kafkaJobMessage) (jobExecutionResult, error) {
	payload, err := payloadToMap(job.Payload)
	if err != nil {
		return jobExecutionResult{}, fmt.Errorf("invalid %s payload: %w", job.JobType, err)
	}

	return jobExecutionResult{
		Input:   payload,
		Output:  map[string]any{"status": "not_implemented", "echo": payload},
		Message: "generic job scaffold completed",
	}, nil
}

// UpsertStatus writes canonical status fields into Redis and refreshes key TTL.
func (s *redisHashStatusStore) UpsertStatus(ctx context.Context, status jobStatusRecord) error {
	key := jobStatusKey(status.JobID)
	fields := map[string]any{
		"job_id":           status.JobID,
		"trace_id":         status.TraceID,
		"state":            status.State,
		"progress_percent": status.ProgressPercent,
		"updated_at":       status.UpdatedAt.Format(time.RFC3339),
		"message":          status.Message,
	}
	if status.ErrorCode != "" {
		fields["error_code"] = status.ErrorCode
		fields["error_message"] = status.ErrorMessage
	}

	if err := s.client.HSet(ctx, key, fields).Err(); err != nil {
		return err
	}
	if status.ErrorCode == "" {
		if err := s.client.HDel(ctx, key, "error_code", "error_message").Err(); err != nil {
			s.logger.Printf("redis HDEL optional error fields failed key=%s err=%v", key, err)
		}
	}
	if err := s.client.Expire(ctx, key, s.ttl).Err(); err != nil {
		return err
	}
	return nil
}

// UpsertJobResult inserts the first final result for a job and no-ops on replay.
func (s *mongoJobResultStore) UpsertJobResult(ctx context.Context, doc jobResultDocument) (bool, error) {
	res, err := s.collection.UpdateOne(
		ctx,
		bson.M{"job_id": doc.JobID},
		bson.M{"$setOnInsert": doc},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return false, err
	}
	return res.UpsertedCount > 0, nil
}

// ensureIndexes creates required Mongo indexes for idempotent job result writes.
func (s *mongoJobResultStore) ensureIndexes(ctx context.Context) error {
	_, err := s.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "job_id", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("job_id_unique"),
	})
	if err != nil {
		return err
	}
	s.logger.Printf("mongo index ensured collection=%s index=job_id_unique", s.collection.Name())
	return nil
}

// newWeatherClient builds the configured weather provider client.
func newWeatherClient(cfg config, logger *log.Logger) (weatherClient, error) {
	mock := &mockWeatherClient{}
	if cfg.weatherProvider == "mock" {
		logger.Println("weather provider configured to mock")
		return mock, nil
	}
	if cfg.weatherProvider == "openmeteo" {
		primary := &openMeteoWeatherClient{
			httpClient:   &http.Client{Timeout: cfg.weatherHTTPTimeout},
			geocodingURL: cfg.weatherGeocodingURL,
			forecastURL:  cfg.weatherForecastURL,
			logger:       logger,
		}
		if cfg.weatherUseMockFallback {
			logger.Println("weather provider configured to openmeteo with mock fallback")
			return &fallbackWeatherClient{primary: primary, fallback: mock, logger: logger}, nil
		}
		logger.Println("weather provider configured to openmeteo")
		return primary, nil
	}
	return nil, fmt.Errorf("unsupported WEATHER_PROVIDER value: %s", cfg.weatherProvider)
}

// Fetch retrieves a weather observation from the primary client and falls back if enabled.
func (c *fallbackWeatherClient) Fetch(ctx context.Context, payload weatherPayload) (weatherObservation, error) {
	obs, err := c.primary.Fetch(ctx, payload)
	if err == nil {
		return obs, nil
	}
	c.logger.Printf("primary weather provider failed city=%s err=%v; using mock fallback", payload.City, err)
	return c.fallback.Fetch(ctx, payload)
}

// Fetch returns deterministic mock weather values for local development.
func (c *mockWeatherClient) Fetch(_ context.Context, payload weatherPayload) (weatherObservation, error) {
	temp := 21.0
	if payload.Units == "imperial" {
		temp = 70.0
	}
	return weatherObservation{
		Temperature: temp,
		Condition:   "Partly Cloudy",
		HumidityPct: 58,
		WindKPH:     12.4,
		Provider:    "mock",
	}, nil
}

// Fetch retrieves weather data from Open-Meteo geocoding and forecast endpoints.
func (c *openMeteoWeatherClient) Fetch(ctx context.Context, payload weatherPayload) (weatherObservation, error) {
	lat, lon, err := c.lookupCoordinates(ctx, payload.City, payload.CountryCode)
	if err != nil {
		return weatherObservation{}, err
	}
	obs, err := c.fetchCurrentWeather(ctx, lat, lon, payload.Units)
	if err != nil {
		return weatherObservation{}, err
	}
	obs.Provider = "open-meteo"
	return obs, nil
}

// lookupCoordinates resolves city text into latitude/longitude using Open-Meteo geocoding.
func (c *openMeteoWeatherClient) lookupCoordinates(ctx context.Context, city, countryCode string) (float64, float64, error) {
	q := url.Values{}
	q.Set("name", city)
	q.Set("count", "1")
	q.Set("language", "en")
	q.Set("format", "json")
	if countryCode != "" {
		q.Set("countryCode", countryCode)
	}

	endpoint := c.geocodingURL + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, 0, err
	}

	var resp struct {
		Results []struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := c.doJSON(req, &resp); err != nil {
		return 0, 0, err
	}
	if len(resp.Results) == 0 {
		return 0, 0, fmt.Errorf("no coordinates found for city=%s country_code=%s", city, countryCode)
	}
	return resp.Results[0].Latitude, resp.Results[0].Longitude, nil
}

// fetchCurrentWeather retrieves current weather and normalizes fields into contract output.
func (c *openMeteoWeatherClient) fetchCurrentWeather(ctx context.Context, lat, lon float64, units string) (weatherObservation, error) {
	temperatureUnit := "celsius"
	if units == "imperial" {
		temperatureUnit = "fahrenheit"
	}

	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', 6, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', 6, 64))
	q.Set("current", "temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m")
	q.Set("temperature_unit", temperatureUnit)
	q.Set("wind_speed_unit", "kmh")
	q.Set("timezone", "UTC")

	endpoint := c.forecastURL + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return weatherObservation{}, err
	}

	var resp struct {
		Current struct {
			Temperature2M     float64 `json:"temperature_2m"`
			RelativeHumidity2 float64 `json:"relative_humidity_2m"`
			WeatherCode       int     `json:"weather_code"`
			WindSpeed10M      float64 `json:"wind_speed_10m"`
		} `json:"current"`
	}
	if err := c.doJSON(req, &resp); err != nil {
		return weatherObservation{}, err
	}

	return weatherObservation{
		Temperature: resp.Current.Temperature2M,
		Condition:   weatherCodeToCondition(resp.Current.WeatherCode),
		HumidityPct: int(resp.Current.RelativeHumidity2),
		WindKPH:     resp.Current.WindSpeed10M,
	}, nil
}

// doJSON performs an HTTP request and decodes a successful JSON response body.
func (c *openMeteoWeatherClient) doJSON(req *http.Request, dst any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("weather provider returned status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return err
	}
	return nil
}

// weatherCodeToCondition maps Open-Meteo weather codes to readable conditions.
func weatherCodeToCondition(code int) string {
	switch code {
	case 0:
		return "Clear"
	case 1, 2:
		return "Partly Cloudy"
	case 3:
		return "Overcast"
	case 45, 48:
		return "Fog"
	case 51, 53, 55, 56, 57:
		return "Drizzle"
	case 61, 63, 65, 66, 67, 80, 81, 82:
		return "Rain"
	case 71, 73, 75, 77, 85, 86:
		return "Snow"
	case 95, 96, 99:
		return "Thunderstorm"
	default:
		return "Unknown"
	}
}

// decodeProgressCheckRequest validates and parses a RabbitMQ status request body.
func decodeProgressCheckRequest(raw []byte) (progressCheckRequest, error) {
	var req progressCheckRequest
	if err := decodeJSONObject(raw, &req); err != nil {
		return progressCheckRequest{}, err
	}

	req.JobID = strings.TrimSpace(req.JobID)
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.RequestedAt = strings.TrimSpace(req.RequestedAt)

	if req.JobID == "" {
		return progressCheckRequest{}, errors.New("job_id is required")
	}
	if req.RequestID == "" {
		return progressCheckRequest{}, errors.New("request_id is required")
	}
	if req.RequestedAt == "" {
		return progressCheckRequest{}, errors.New("requested_at is required")
	}
	if _, err := time.Parse(time.RFC3339, req.RequestedAt); err != nil {
		return progressCheckRequest{}, fmt.Errorf("requested_at must be RFC3339: %w", err)
	}
	return req, nil
}

// decodeKafkaJobMessage validates and parses the canonical Kafka envelope.
func decodeKafkaJobMessage(raw []byte) (kafkaJobMessage, error) {
	if len(raw) == 0 {
		return kafkaJobMessage{}, errors.New("empty message payload")
	}

	var msg kafkaJobMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return kafkaJobMessage{}, fmt.Errorf("decode failed: %w", err)
	}

	msg.SchemaVersion = strings.TrimSpace(msg.SchemaVersion)
	msg.JobID = strings.TrimSpace(msg.JobID)
	msg.JobType = strings.ToLower(strings.TrimSpace(msg.JobType))
	msg.SubmittedAt = strings.TrimSpace(msg.SubmittedAt)
	msg.TraceID = strings.TrimSpace(msg.TraceID)

	if msg.SchemaVersion != "" && msg.SchemaVersion != "1.0" {
		return kafkaJobMessage{}, fmt.Errorf("unsupported schema_version: %s", msg.SchemaVersion)
	}
	if msg.JobID == "" {
		return kafkaJobMessage{}, errors.New("job_id is required")
	}
	if msg.JobType == "" {
		return kafkaJobMessage{}, errors.New("job_type is required")
	}
	if msg.SubmittedAt == "" {
		return kafkaJobMessage{}, errors.New("submitted_at is required")
	}
	if msg.TraceID == "" {
		return kafkaJobMessage{}, errors.New("trace_id is required")
	}
	if len(msg.Payload) == 0 {
		return kafkaJobMessage{}, errors.New("payload is required")
	}
	if _, err := time.Parse(time.RFC3339, msg.SubmittedAt); err != nil {
		return kafkaJobMessage{}, fmt.Errorf("submitted_at must be RFC3339: %w", err)
	}

	return msg, nil
}

// decodeJSONObject decodes one JSON value and rejects trailing tokens.
func decodeJSONObject(raw []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("multiple JSON values are not allowed")
	}
	return nil
}

// payloadToMap decodes payload JSON into a map for generic handlers/persistence.
func payloadToMap(raw []byte) (map[string]any, error) {
	var payload map[string]any
	if err := decodeJSONObject(raw, &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, errors.New("payload must be a JSON object")
	}
	return payload, nil
}

// normalizeJobTypes validates configured job types and removes duplicates.
func normalizeJobTypes(jobTypes []string) ([]string, error) {
	if len(jobTypes) == 0 {
		return nil, errors.New("WORKER_JOB_TYPES must include at least one value")
	}

	normalized := make([]string, 0, len(jobTypes))
	for _, jobType := range jobTypes {
		jobType = strings.ToLower(strings.TrimSpace(jobType))
		if jobType == "" {
			continue
		}
		if !slices.Contains(supportedJobTypes, jobType) {
			return nil, fmt.Errorf("unsupported WORKER_JOB_TYPES value: %s", jobType)
		}
		if !slices.Contains(normalized, jobType) {
			normalized = append(normalized, jobType)
		}
	}
	if len(normalized) == 0 {
		return nil, errors.New("WORKER_JOB_TYPES must include at least one supported value")
	}
	return normalized, nil
}

// resolveKafkaTopics returns concrete topics from override or template + job types.
func resolveKafkaTopics(jobTypes []string, overrideTopic, topicTemplate string) []string {
	if overrideTopic != "" {
		return []string{overrideTopic}
	}

	topics := make([]string, 0, len(jobTypes))
	for _, jobType := range jobTypes {
		topic := topicTemplate
		if strings.Contains(topicTemplate, "%s") {
			topic = fmt.Sprintf(topicTemplate, jobType)
		}
		if !slices.Contains(topics, topic) {
			topics = append(topics, topic)
		}
	}
	return topics
}

// parseProgressPercent converts status progress values to bounded integer percentages.
func parseProgressPercent(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

// progressMessageForJobType returns a status message for the 50% processing milestone.
func progressMessageForJobType(jobType string) string {
	if jobType == "weather" {
		return "calling weather provider"
	}
	return "processing job"
}

// jobStatusKey returns the canonical Redis key for transient job status.
func jobStatusKey(jobID string) string {
	return fmt.Sprintf("job:%s:status", jobID)
}

// envOrDefault returns an environment variable value or fallback if empty.
func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		appLogger.Printf("env %s not set; using default=%q", key, fallback)
		return fallback
	}
	return value
}

// parseCSVEnv parses a comma-delimited environment variable into a string slice.
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

// parseIntEnv parses an integer environment variable with fallback.
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

// parseDurationEnv parses a duration environment variable with fallback.
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

// parseBoolEnv parses a boolean environment variable with fallback.
func parseBoolEnv(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		appLogger.Printf("env %s not set; using default bool=%t", key, fallback)
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		appLogger.Printf("env %s invalid bool value=%q err=%v", key, raw, err)
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return value, nil
}
