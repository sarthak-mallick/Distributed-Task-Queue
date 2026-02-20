package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"distributed-task-queue/api/internal/apiworkergrpc"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var graphqlWSUpgrader = websocket.Upgrader{
	Subprotocols: []string{"graphql-transport-ws"},
	CheckOrigin: func(*http.Request) bool {
		return true
	},
}

// graphQLRequest models one HTTP/WS GraphQL operation payload.
type graphQLRequest struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

// graphQLResponse is the standard GraphQL response envelope.
type graphQLResponse struct {
	Data   map[string]any `json:"data,omitempty"`
	Errors []graphQLError `json:"errors,omitempty"`
}

// graphQLError models one GraphQL error message item.
type graphQLError struct {
	Message string `json:"message"`
}

// graphQLWSMessage models one graphql-transport-ws protocol frame.
type graphQLWSMessage struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// handleLegacyRESTDisabled disables Week 1 REST paths in Week 2 canonical flow.
func (a *app) handleLegacyRESTDisabled(w http.ResponseWriter, r *http.Request) {
	a.logger.Printf("legacy REST endpoint rejected method=%s path=%s", r.Method, r.URL.Path)
	writeJSON(w, http.StatusGone, errorResponse{Error: "legacy REST endpoints are disabled; use /graphql"})
}

// handleGraphQL serves GraphQL mutation/query operations over HTTP POST.
func (a *app) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()
	operation := "unknown"
	success := false
	defer func() {
		apiMetricsState.recordGraphQLHTTPRequest(operation, time.Since(startedAt), success)
	}()

	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	var req graphQLRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, graphQLErrorResponse(err.Error()))
		return
	}
	operation = resolveGraphQLOperation(req)

	result, err := a.executeGraphQLOperation(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusOK, graphQLErrorResponse(err.Error()))
		return
	}
	success = true
	writeJSON(w, http.StatusOK, graphQLResponse{Data: result})
}

// executeGraphQLOperation routes one GraphQL operation to submit/status handlers.
func (a *app) executeGraphQLOperation(ctx context.Context, req graphQLRequest) (map[string]any, error) {
	op := resolveGraphQLOperation(req)
	switch op {
	case "submitJob":
		jobType, payload, err := parseGraphQLSubmitInput(req.Variables)
		if err != nil {
			return nil, err
		}

		normalized, err := validateSubmitJobRequest(submitJobRequest{JobType: jobType, Payload: payload})
		if err != nil {
			return nil, err
		}

		jobID, traceID, submittedAt, err := a.enqueueJob(ctx, normalized.JobType, normalized.Payload)
		if err != nil {
			return nil, err
		}
		apiMetricsState.recordJobSubmission()

		return map[string]any{
			"submitJob": map[string]any{
				"jobId":       jobID,
				"traceId":     traceID,
				"jobType":     normalized.JobType,
				"state":       "queued",
				"submittedAt": submittedAt.Format(time.RFC3339),
				"message":     "job accepted",
			},
		}, nil
	case "jobStatus":
		jobID, err := extractGraphQLJobID(req.Variables)
		if err != nil {
			return nil, err
		}
		status, err := a.getJobStatusViaGRPC(ctx, jobID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"jobStatus": graphQLStatusMap(status)}, nil
	default:
		return nil, errors.New("unsupported GraphQL operation; use submitJob mutation or jobStatus query")
	}
}

// resolveGraphQLOperation identifies operation target from operationName/query text.
func resolveGraphQLOperation(req graphQLRequest) string {
	operationName := strings.TrimSpace(req.OperationName)
	if operationName != "" {
		switch operationName {
		case "SubmitJob", "submitJob":
			return "submitJob"
		case "JobStatus", "jobStatus":
			return "jobStatus"
		case "JobProgress", "jobProgress":
			return "jobProgress"
		}
	}

	query := strings.ToLower(req.Query)
	switch {
	case strings.Contains(query, "submitjob"):
		return "submitJob"
	case strings.Contains(query, "jobstatus"):
		return "jobStatus"
	case strings.Contains(query, "jobprogress"):
		return "jobProgress"
	default:
		return ""
	}
}

// parseGraphQLSubmitInput extracts submit mutation input fields and normalizes payload keys.
func parseGraphQLSubmitInput(vars map[string]any) (string, json.RawMessage, error) {
	if vars == nil {
		return "", nil, errors.New("variables are required")
	}

	rawInput, ok := vars["input"]
	if !ok {
		return "", nil, errors.New("variables.input is required")
	}

	input, ok := rawInput.(map[string]any)
	if !ok {
		return "", nil, errors.New("variables.input must be an object")
	}

	jobType, _ := input["jobType"].(string)
	jobType = strings.TrimSpace(jobType)
	if jobType == "" {
		return "", nil, errors.New("input.jobType is required")
	}

	payload, ok := input["payload"]
	if !ok {
		return "", nil, errors.New("input.payload is required")
	}

	normalizedPayload := normalizeGraphQLPayload(payload)
	body, err := json.Marshal(normalizedPayload)
	if err != nil {
		return "", nil, fmt.Errorf("input.payload encode failed: %w", err)
	}
	return jobType, body, nil
}

// normalizeGraphQLPayload maps optional camelCase payload aliases to Week 1 contract keys.
func normalizeGraphQLPayload(payload any) any {
	m, ok := payload.(map[string]any)
	if !ok {
		return payload
	}
	if _, hasSnake := m["country_code"]; !hasSnake {
		if v, ok := m["countryCode"]; ok {
			m["country_code"] = v
		}
	}
	return m
}

// extractGraphQLJobID returns validated UUID job_id from variables map.
func extractGraphQLJobID(vars map[string]any) (string, error) {
	if vars == nil {
		return "", errors.New("variables are required")
	}
	jobID, _ := vars["jobId"].(string)
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return "", errors.New("variables.jobId is required")
	}
	if _, err := uuid.Parse(jobID); err != nil {
		return "", errors.New("variables.jobId must be a valid UUID")
	}
	return jobID, nil
}

// getJobStatusViaGRPC fetches latest status from worker through Week 2 gRPC path.
func (a *app) getJobStatusViaGRPC(ctx context.Context, jobID string) (progressCheckReply, error) {
	if a.workerGRPC == nil {
		return progressCheckReply{}, errors.New("worker gRPC client is not configured")
	}

	reply, err := a.workerGRPC.GetJobStatus(ctx, &apiworkergrpc.GetJobStatusRequest{JobID: jobID})
	if err != nil {
		return progressCheckReply{}, fmt.Errorf("worker status RPC failed: %w", err)
	}
	return progressCheckReply{
		JobID:           reply.JobID,
		State:           reply.State,
		ProgressPercent: int(reply.ProgressPercent),
		Message:         reply.Message,
		Timestamp:       reply.Timestamp,
	}, nil
}

// graphQLStatusMap projects internal status fields into GraphQL response naming.
func graphQLStatusMap(status progressCheckReply) map[string]any {
	return map[string]any{
		"jobId":           status.JobID,
		"state":           status.State,
		"progressPercent": status.ProgressPercent,
		"message":         status.Message,
		"timestamp":       status.Timestamp,
	}
}

// graphQLErrorResponse wraps one error string into GraphQL error envelope.
func graphQLErrorResponse(message string) graphQLResponse {
	return graphQLResponse{Errors: []graphQLError{{Message: message}}}
}

// handleGraphQLWebSocket serves GraphQL subscription events using graphql-transport-ws.
func (a *app) handleGraphQLWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	conn, err := graphqlWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		a.logger.Printf("graphql ws upgrade failed: %v", err)
		return
	}
	apiMetricsState.recordGraphQLSubscriptionConnection()

	sessionCtx, cancel := context.WithCancel(context.Background())
	session := &graphQLSubscriptionSession{
		app:         a,
		conn:        conn,
		sessionCtx:  sessionCtx,
		sessionStop: cancel,
		subCancels:  map[string]context.CancelFunc{},
	}
	defer session.close()

	for {
		var msg graphQLWSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				a.logger.Printf("graphql ws read failed: %v", err)
			}
			return
		}

		switch msg.Type {
		case "connection_init":
			_ = session.write(graphQLWSMessage{Type: "connection_ack"})
		case "ping":
			_ = session.write(graphQLWSMessage{Type: "pong"})
		case "subscribe":
			session.startSubscription(msg)
		case "complete":
			session.stopSubscription(msg.ID)
		default:
			session.writeError(msg.ID, "unsupported graphql ws message type")
		}
	}
}

// graphQLSubscriptionSession owns one websocket connection and active subscription goroutines.
type graphQLSubscriptionSession struct {
	app         *app
	conn        *websocket.Conn
	sessionCtx  context.Context
	sessionStop context.CancelFunc
	writeMu     sync.Mutex
	subsMu      sync.Mutex
	subCancels  map[string]context.CancelFunc
}

// close stops all active subscriptions and closes websocket resources.
func (s *graphQLSubscriptionSession) close() {
	s.sessionStop()
	s.subsMu.Lock()
	for _, cancel := range s.subCancels {
		cancel()
	}
	s.subCancels = map[string]context.CancelFunc{}
	s.subsMu.Unlock()
	_ = s.conn.Close()
}

// write sends one protocol message with synchronized websocket writes.
func (s *graphQLSubscriptionSession) write(msg graphQLWSMessage) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteJSON(msg)
}

// startSubscription starts or replaces one subscription stream for the given operation id.
func (s *graphQLSubscriptionSession) startSubscription(msg graphQLWSMessage) {
	if strings.TrimSpace(msg.ID) == "" {
		s.writeError("", "subscription id is required")
		return
	}

	var req graphQLRequest
	if err := decodeJSONBytes(msg.Payload, &req); err != nil {
		s.writeError(msg.ID, "invalid subscription payload")
		return
	}
	if resolveGraphQLOperation(req) != "jobProgress" {
		s.writeError(msg.ID, "only jobProgress subscription is supported")
		return
	}
	jobID, err := extractGraphQLJobID(req.Variables)
	if err != nil {
		s.writeError(msg.ID, err.Error())
		return
	}

	s.stopSubscription(msg.ID)

	subCtx, cancel := context.WithCancel(s.sessionCtx)
	s.subsMu.Lock()
	s.subCancels[msg.ID] = cancel
	s.subsMu.Unlock()

	go s.runProgressSubscription(subCtx, msg.ID, jobID)
}

// stopSubscription cancels one active subscription by operation id.
func (s *graphQLSubscriptionSession) stopSubscription(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	s.subsMu.Lock()
	cancel, ok := s.subCancels[id]
	if ok {
		delete(s.subCancels, id)
	}
	s.subsMu.Unlock()
	if ok {
		cancel()
	}
}

// runProgressSubscription streams worker gRPC updates into graphql-transport-ws next frames.
func (s *graphQLSubscriptionSession) runProgressSubscription(ctx context.Context, id, jobID string) {
	stream, err := s.app.workerGRPC.SubscribeJobProgress(ctx, &apiworkergrpc.SubscribeJobProgressRequest{
		JobID:          jobID,
		PollIntervalMS: int32(s.app.cfg.subscriptionPoll / time.Millisecond),
	})
	if err != nil {
		s.writeError(id, fmt.Sprintf("subscription start failed: %v", err))
		_ = s.write(graphQLWSMessage{ID: id, Type: "complete"})
		return
	}

	for {
		reply, err := stream.Recv()
		if err != nil {
			if err != io.EOF && ctx.Err() == nil {
				s.writeError(id, fmt.Sprintf("subscription stream failed: %v", err))
			}
			_ = s.write(graphQLWSMessage{ID: id, Type: "complete"})
			return
		}

		nextPayload, err := json.Marshal(map[string]any{
			"data": map[string]any{
				"jobProgress": graphQLStatusMap(progressCheckReply{
					JobID:           reply.JobID,
					State:           reply.State,
					ProgressPercent: int(reply.ProgressPercent),
					Message:         reply.Message,
					Timestamp:       reply.Timestamp,
				}),
			},
		})
		if err != nil {
			s.writeError(id, "subscription payload encode failed")
			_ = s.write(graphQLWSMessage{ID: id, Type: "complete"})
			return
		}

		if err := s.write(graphQLWSMessage{ID: id, Type: "next", Payload: nextPayload}); err != nil {
			return
		}
		if isTerminalSubscriptionState(reply.State) {
			_ = s.write(graphQLWSMessage{ID: id, Type: "complete"})
			return
		}
	}
}

// writeError sends graphql-transport-ws error payloads for one operation id.
func (s *graphQLSubscriptionSession) writeError(id, message string) {
	payload, err := json.Marshal([]graphQLError{{Message: message}})
	if err != nil {
		return
	}
	_ = s.write(graphQLWSMessage{ID: id, Type: "error", Payload: payload})
}

// isTerminalSubscriptionState returns true once no further updates should be emitted.
func isTerminalSubscriptionState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "completed", "failed", "not_found":
		return true
	default:
		return false
	}
}
