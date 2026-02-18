package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"distributed-task-queue/api/internal/apiworkergrpc"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// fakeAPIWorkerClient implements API worker RPC calls for GraphQL handler tests.
type fakeAPIWorkerClient struct {
	getStatusFn         func(context.Context, *apiworkergrpc.GetJobStatusRequest) (*apiworkergrpc.JobStatusReply, error)
	subscribeProgressFn func(context.Context, *apiworkergrpc.SubscribeJobProgressRequest) (apiworkergrpc.APIWorker_SubscribeJobProgressClient, error)
}

// GetJobStatus returns scripted worker status responses.
func (f *fakeAPIWorkerClient) GetJobStatus(ctx context.Context, req *apiworkergrpc.GetJobStatusRequest, _ ...grpc.CallOption) (*apiworkergrpc.JobStatusReply, error) {
	if f.getStatusFn == nil {
		return nil, io.EOF
	}
	return f.getStatusFn(ctx, req)
}

// SubscribeJobProgress returns a scripted stream for subscription tests.
func (f *fakeAPIWorkerClient) SubscribeJobProgress(ctx context.Context, req *apiworkergrpc.SubscribeJobProgressRequest, _ ...grpc.CallOption) (apiworkergrpc.APIWorker_SubscribeJobProgressClient, error) {
	if f.subscribeProgressFn == nil {
		return nil, io.EOF
	}
	return f.subscribeProgressFn(ctx, req)
}

// fakeProgressClientStream provides scripted stream frames for subscription tests.
type fakeProgressClientStream struct {
	ctx     context.Context
	replies []*apiworkergrpc.JobStatusReply
	idx     int
}

// Recv returns scripted status frames and then io.EOF.
func (f *fakeProgressClientStream) Recv() (*apiworkergrpc.JobStatusReply, error) {
	if f.idx >= len(f.replies) {
		return nil, io.EOF
	}
	reply := f.replies[f.idx]
	f.idx++
	return reply, nil
}

// Header satisfies grpc.ClientStream.
func (f *fakeProgressClientStream) Header() (metadata.MD, error) {
	return metadata.MD{}, nil
}

// Trailer satisfies grpc.ClientStream.
func (f *fakeProgressClientStream) Trailer() metadata.MD {
	return metadata.MD{}
}

// CloseSend satisfies grpc.ClientStream.
func (f *fakeProgressClientStream) CloseSend() error {
	return nil
}

// Context satisfies grpc.ClientStream.
func (f *fakeProgressClientStream) Context() context.Context {
	if f.ctx == nil {
		return context.Background()
	}
	return f.ctx
}

// SendMsg satisfies grpc.ClientStream.
func (f *fakeProgressClientStream) SendMsg(any) error {
	return nil
}

// RecvMsg satisfies grpc.ClientStream.
func (f *fakeProgressClientStream) RecvMsg(any) error {
	return nil
}

func TestResolveGraphQLOperation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  graphQLRequest
		want string
	}{
		{name: "operation name submit", req: graphQLRequest{OperationName: "SubmitJob"}, want: "submitJob"},
		{name: "query fallback status", req: graphQLRequest{Query: "query { jobStatus(jobId:\"x\"){state} }"}, want: "jobStatus"},
		{name: "query fallback subscription", req: graphQLRequest{Query: "subscription { jobProgress(jobId:\"x\"){state} }"}, want: "jobProgress"},
		{name: "unknown", req: graphQLRequest{Query: "query { healthz }"}, want: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveGraphQLOperation(tt.req); got != tt.want {
				t.Fatalf("resolveGraphQLOperation() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseGraphQLSubmitInput(t *testing.T) {
	t.Parallel()

	jobType, payload, err := parseGraphQLSubmitInput(map[string]any{
		"input": map[string]any{
			"jobType": "weather",
			"payload": map[string]any{
				"city":        "Austin",
				"countryCode": "US",
				"units":       "metric",
			},
		},
	})
	if err != nil {
		t.Fatalf("parseGraphQLSubmitInput() error = %v", err)
	}
	if jobType != "weather" {
		t.Fatalf("jobType = %q, want weather", jobType)
	}

	var payloadMap map[string]any
	if err := json.Unmarshal(payload, &payloadMap); err != nil {
		t.Fatalf("payload unmarshal failed: %v", err)
	}
	if payloadMap["country_code"] != "US" {
		t.Fatalf("country_code normalization missing: %v", payloadMap)
	}
}

func TestExtractGraphQLJobID(t *testing.T) {
	t.Parallel()

	if _, err := extractGraphQLJobID(map[string]any{"jobId": "not-a-uuid"}); err == nil {
		t.Fatal("expected invalid UUID error")
	}

	id, err := extractGraphQLJobID(map[string]any{"jobId": "6aab8fca-7059-40c4-97d4-53f55fd5bf67"})
	if err != nil {
		t.Fatalf("extractGraphQLJobID() error = %v", err)
	}
	if id != "6aab8fca-7059-40c4-97d4-53f55fd5bf67" {
		t.Fatalf("id = %q, want canonical UUID", id)
	}
}

func TestHandleLegacyRESTDisabled(t *testing.T) {
	t.Parallel()

	a := &app{logger: log.New(io.Discard, "", 0)}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)

	a.handleLegacyRESTDisabled(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGone)
	}
}

func TestHandleGraphQLWebSocket_SubscriptionStream(t *testing.T) {
	t.Parallel()

	client := &fakeAPIWorkerClient{
		subscribeProgressFn: func(_ context.Context, req *apiworkergrpc.SubscribeJobProgressRequest) (apiworkergrpc.APIWorker_SubscribeJobProgressClient, error) {
			if req.JobID != "6aab8fca-7059-40c4-97d4-53f55fd5bf67" {
				t.Fatalf("unexpected job id: %s", req.JobID)
			}
			return &fakeProgressClientStream{
				replies: []*apiworkergrpc.JobStatusReply{
					{JobID: req.JobID, State: "running", ProgressPercent: 50, Message: "working", Timestamp: time.Now().UTC().Format(time.RFC3339)},
					{JobID: req.JobID, State: "completed", ProgressPercent: 100, Message: "done", Timestamp: time.Now().UTC().Format(time.RFC3339)},
				},
			}, nil
		},
	}

	a := &app{
		cfg:        config{subscriptionPoll: 5 * time.Millisecond},
		logger:     log.New(io.Discard, "", 0),
		workerGRPC: client,
	}

	ts := httptest.NewServer(a.routes())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/graphql/ws"
	dialer := websocket.Dialer{Subprotocols: []string{"graphql-transport-ws"}}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	if err := conn.WriteJSON(graphQLWSMessage{Type: "connection_init"}); err != nil {
		t.Fatalf("write connection_init failed: %v", err)
	}

	var ack graphQLWSMessage
	if err := conn.ReadJSON(&ack); err != nil {
		t.Fatalf("read ack failed: %v", err)
	}
	if ack.Type != "connection_ack" {
		t.Fatalf("ack type = %q, want connection_ack", ack.Type)
	}

	subPayload, err := json.Marshal(graphQLRequest{
		Query:     "subscription JobProgress($jobId: ID!){jobProgress(jobId:$jobId){jobId state progressPercent message timestamp}}",
		Variables: map[string]any{"jobId": "6aab8fca-7059-40c4-97d4-53f55fd5bf67"},
	})
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}

	if err := conn.WriteJSON(graphQLWSMessage{ID: "sub-1", Type: "subscribe", Payload: subPayload}); err != nil {
		t.Fatalf("write subscribe failed: %v", err)
	}

	seenNext := 0
	seenComplete := false
	for i := 0; i < 5; i++ {
		var msg graphQLWSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("read subscription message failed: %v", err)
		}

		switch msg.Type {
		case "next":
			seenNext++
		case "complete":
			seenComplete = true
		}
		if seenNext >= 2 && seenComplete {
			break
		}
	}

	if seenNext < 2 || !seenComplete {
		t.Fatalf("expected at least 2 next events and a complete event; got next=%d complete=%t", seenNext, seenComplete)
	}
}
