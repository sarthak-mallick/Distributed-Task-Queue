package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// workerMetrics stores process-local worker observability counters exported via /metrics.
type workerMetrics struct {
	jobAttemptsTotal              atomic.Uint64
	jobCompletedTotal             atomic.Uint64
	jobFailedTotal                atomic.Uint64
	jobDroppedTotal               atomic.Uint64
	jobDurationNanosTotal         atomic.Uint64
	jobDurationSamplesTotal       atomic.Uint64
	kafkaFetchErrorsTotal         atomic.Uint64
	grpcStatusRequestsTotal       atomic.Uint64
	grpcSubscriptionsTotal        atomic.Uint64
	rabbitProgressRequestsTotal   atomic.Uint64
	gracefulShutdownRequestsTotal atomic.Uint64
	currentJobsInFlight           atomic.Int64
}

// recordJobAttemptStart records one accepted worker processing attempt.
func (m *workerMetrics) recordJobAttemptStart() {
	m.jobAttemptsTotal.Add(1)
	m.currentJobsInFlight.Add(1)
}

// recordJobAttemptEnd records final outcome and duration for one processing attempt.
func (m *workerMetrics) recordJobAttemptEnd(outcome string, duration time.Duration) {
	m.currentJobsInFlight.Add(-1)
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "completed":
		m.jobCompletedTotal.Add(1)
	default:
		m.jobFailedTotal.Add(1)
	}
	m.jobDurationNanosTotal.Add(uint64(duration.Nanoseconds()))
	m.jobDurationSamplesTotal.Add(1)
}

// recordDroppedMessage increments invalid/unsupported message drop counters.
func (m *workerMetrics) recordDroppedMessage() {
	m.jobDroppedTotal.Add(1)
}

// recordKafkaFetchError increments fetch-loop error counters.
func (m *workerMetrics) recordKafkaFetchError() {
	m.kafkaFetchErrorsTotal.Add(1)
}

// recordGRPCStatusRequest increments unary status RPC request counters.
func (m *workerMetrics) recordGRPCStatusRequest() {
	m.grpcStatusRequestsTotal.Add(1)
}

// recordGRPCSubscription increments subscription RPC request counters.
func (m *workerMetrics) recordGRPCSubscription() {
	m.grpcSubscriptionsTotal.Add(1)
}

// recordRabbitProgressRequest increments RabbitMQ request-reply progress counters.
func (m *workerMetrics) recordRabbitProgressRequest() {
	m.rabbitProgressRequestsTotal.Add(1)
}

// recordGracefulShutdownRequest increments graceful shutdown trigger counters.
func (m *workerMetrics) recordGracefulShutdownRequest() {
	m.gracefulShutdownRequestsTotal.Add(1)
}

// renderPrometheus renders Prometheus text exposition for worker counters.
func (m *workerMetrics) renderPrometheus() string {
	durationSumSeconds := float64(m.jobDurationNanosTotal.Load()) / float64(time.Second)
	averageDurationSeconds := 0.0
	if samples := m.jobDurationSamplesTotal.Load(); samples > 0 {
		averageDurationSeconds = durationSumSeconds / float64(samples)
	}

	var b strings.Builder
	writeWorkerCounterMetric(&b, "dtq_worker_job_attempts_total", "Total worker job processing attempts.", m.jobAttemptsTotal.Load())
	writeWorkerCounterMetric(&b, "dtq_worker_job_completed_total", "Total worker job attempts that completed successfully.", m.jobCompletedTotal.Load())
	writeWorkerCounterMetric(&b, "dtq_worker_job_failed_total", "Total worker job attempts that failed.", m.jobFailedTotal.Load())
	writeWorkerCounterMetric(&b, "dtq_worker_job_dropped_total", "Total worker messages dropped before processing (invalid/unsupported).", m.jobDroppedTotal.Load())
	writeWorkerCounterMetric(&b, "dtq_worker_kafka_fetch_errors_total", "Total Kafka fetch-loop errors observed by worker.", m.kafkaFetchErrorsTotal.Load())
	writeWorkerCounterMetric(&b, "dtq_worker_grpc_status_requests_total", "Total worker GetJobStatus gRPC requests.", m.grpcStatusRequestsTotal.Load())
	writeWorkerCounterMetric(&b, "dtq_worker_grpc_subscriptions_total", "Total worker SubscribeJobProgress gRPC requests.", m.grpcSubscriptionsTotal.Load())
	writeWorkerCounterMetric(&b, "dtq_worker_rabbit_progress_requests_total", "Total worker RabbitMQ progress requests handled.", m.rabbitProgressRequestsTotal.Load())
	writeWorkerCounterMetric(&b, "dtq_worker_graceful_shutdown_requests_total", "Total worker graceful shutdown requests received.", m.gracefulShutdownRequestsTotal.Load())
	writeWorkerCounterMetric(&b, "dtq_worker_job_duration_seconds_count", "Total worker job-duration samples.", m.jobDurationSamplesTotal.Load())
	b.WriteString("# HELP dtq_worker_jobs_in_flight Current in-flight worker jobs.\n")
	b.WriteString("# TYPE dtq_worker_jobs_in_flight gauge\n")
	b.WriteString(fmt.Sprintf("dtq_worker_jobs_in_flight %d\n", m.currentJobsInFlight.Load()))
	b.WriteString("# HELP dtq_worker_job_duration_seconds_sum Total worker job duration seconds.\n")
	b.WriteString("# TYPE dtq_worker_job_duration_seconds_sum counter\n")
	b.WriteString(fmt.Sprintf("dtq_worker_job_duration_seconds_sum %.6f\n", durationSumSeconds))
	b.WriteString("# HELP dtq_worker_job_duration_seconds_average Average worker job duration seconds.\n")
	b.WriteString("# TYPE dtq_worker_job_duration_seconds_average gauge\n")
	b.WriteString(fmt.Sprintf("dtq_worker_job_duration_seconds_average %.6f\n", averageDurationSeconds))
	return b.String()
}

// writeWorkerCounterMetric writes one Prometheus counter entry.
func writeWorkerCounterMetric(builder *strings.Builder, name, help string, value uint64) {
	builder.WriteString("# HELP ")
	builder.WriteString(name)
	builder.WriteByte(' ')
	builder.WriteString(help)
	builder.WriteByte('\n')
	builder.WriteString("# TYPE ")
	builder.WriteString(name)
	builder.WriteString(" counter\n")
	builder.WriteString(name)
	builder.WriteByte(' ')
	builder.WriteString(fmt.Sprintf("%d\n", value))
}
