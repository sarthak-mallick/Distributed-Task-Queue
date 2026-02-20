package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// apiMetrics stores process-local API observability counters exported via /metrics.
type apiMetrics struct {
	graphqlHTTPRequestsTotal             atomic.Uint64
	graphqlHTTPFailuresTotal             atomic.Uint64
	graphqlSubmitRequestsTotal           atomic.Uint64
	graphqlStatusRequestsTotal           atomic.Uint64
	graphqlUnknownRequestsTotal          atomic.Uint64
	graphqlSubscriptionConnectionsTotal  atomic.Uint64
	jobSubmissionsTotal                  atomic.Uint64
	graphqlHTTPRequestDurationNanosTotal atomic.Uint64
	graphqlHTTPRequestDurationCountTotal atomic.Uint64
}

var apiMetricsState = &apiMetrics{}

// recordGraphQLHTTPRequest tracks one GraphQL HTTP request outcome and latency.
func (m *apiMetrics) recordGraphQLHTTPRequest(operation string, duration time.Duration, success bool) {
	m.graphqlHTTPRequestsTotal.Add(1)
	switch normalizeGraphQLOperation(operation) {
	case "submit":
		m.graphqlSubmitRequestsTotal.Add(1)
	case "status":
		m.graphqlStatusRequestsTotal.Add(1)
	default:
		m.graphqlUnknownRequestsTotal.Add(1)
	}
	if !success {
		m.graphqlHTTPFailuresTotal.Add(1)
	}
	m.graphqlHTTPRequestDurationNanosTotal.Add(uint64(duration.Nanoseconds()))
	m.graphqlHTTPRequestDurationCountTotal.Add(1)
}

// recordGraphQLSubscriptionConnection increments active connection acceptance counters.
func (m *apiMetrics) recordGraphQLSubscriptionConnection() {
	m.graphqlSubscriptionConnectionsTotal.Add(1)
}

// recordJobSubmission increments accepted API job submission counters.
func (m *apiMetrics) recordJobSubmission() {
	m.jobSubmissionsTotal.Add(1)
}

// renderPrometheus renders Prometheus text exposition for API counters.
func (m *apiMetrics) renderPrometheus() string {
	durationSumSeconds := float64(m.graphqlHTTPRequestDurationNanosTotal.Load()) / float64(time.Second)
	averageDurationSeconds := 0.0
	if count := m.graphqlHTTPRequestDurationCountTotal.Load(); count > 0 {
		averageDurationSeconds = durationSumSeconds / float64(count)
	}

	var b strings.Builder
	writeCounterMetric(&b, "dtq_api_graphql_http_requests_total", "Total GraphQL HTTP requests handled by API.", m.graphqlHTTPRequestsTotal.Load())
	writeCounterMetric(&b, "dtq_api_graphql_http_failures_total", "Total failed GraphQL HTTP requests handled by API.", m.graphqlHTTPFailuresTotal.Load())
	writeCounterMetric(&b, "dtq_api_graphql_submit_requests_total", "Total GraphQL submitJob requests handled by API.", m.graphqlSubmitRequestsTotal.Load())
	writeCounterMetric(&b, "dtq_api_graphql_status_requests_total", "Total GraphQL jobStatus requests handled by API.", m.graphqlStatusRequestsTotal.Load())
	writeCounterMetric(&b, "dtq_api_graphql_unknown_requests_total", "Total GraphQL requests with unsupported/unknown operation.", m.graphqlUnknownRequestsTotal.Load())
	writeCounterMetric(&b, "dtq_api_graphql_subscription_connections_total", "Total accepted GraphQL WebSocket subscription connections.", m.graphqlSubscriptionConnectionsTotal.Load())
	writeCounterMetric(&b, "dtq_api_job_submissions_total", "Total accepted API job submissions.", m.jobSubmissionsTotal.Load())
	writeCounterMetric(&b, "dtq_api_graphql_http_request_duration_seconds_count", "Total GraphQL HTTP request duration samples.", m.graphqlHTTPRequestDurationCountTotal.Load())
	b.WriteString("# HELP dtq_api_graphql_http_request_duration_seconds_sum Total GraphQL HTTP request duration seconds.\n")
	b.WriteString("# TYPE dtq_api_graphql_http_request_duration_seconds_sum counter\n")
	b.WriteString(fmt.Sprintf("dtq_api_graphql_http_request_duration_seconds_sum %.6f\n", durationSumSeconds))
	b.WriteString("# HELP dtq_api_graphql_http_request_duration_seconds_average Average GraphQL HTTP request duration seconds.\n")
	b.WriteString("# TYPE dtq_api_graphql_http_request_duration_seconds_average gauge\n")
	b.WriteString(fmt.Sprintf("dtq_api_graphql_http_request_duration_seconds_average %.6f\n", averageDurationSeconds))
	return b.String()
}

// normalizeGraphQLOperation maps API operation names into stable metric buckets.
func normalizeGraphQLOperation(operation string) string {
	switch strings.ToLower(strings.TrimSpace(operation)) {
	case "submitjob", "submit":
		return "submit"
	case "jobstatus", "status":
		return "status"
	default:
		return "unknown"
	}
}

// writeCounterMetric writes one Prometheus counter entry.
func writeCounterMetric(builder *strings.Builder, name, help string, value uint64) {
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
