package apiworkergrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

const (
	// ServiceName is the canonical gRPC service name for Week 2 API-worker calls.
	ServiceName = "dtq.v1.APIWorker"
	// jsonCodecName is the registered gRPC content subtype for JSON payloads.
	jsonCodecName = "json"
)

// GetJobStatusRequest asks worker for one job's latest status.
type GetJobStatusRequest struct {
	JobID string `json:"job_id"`
}

// SubscribeJobProgressRequest starts a worker progress stream for one job.
type SubscribeJobProgressRequest struct {
	JobID          string `json:"job_id"`
	PollIntervalMS int32  `json:"poll_interval_ms,omitempty"`
}

// JobStatusReply returns one status snapshot for a job.
type JobStatusReply struct {
	JobID           string `json:"job_id"`
	State           string `json:"state"`
	ProgressPercent int32  `json:"progress_percent"`
	Message         string `json:"message"`
	Timestamp       string `json:"timestamp"`
}

// APIWorkerClient defines client RPC calls used by the API service.
type APIWorkerClient interface {
	GetJobStatus(ctx context.Context, in *GetJobStatusRequest, opts ...grpc.CallOption) (*JobStatusReply, error)
	SubscribeJobProgress(ctx context.Context, in *SubscribeJobProgressRequest, opts ...grpc.CallOption) (APIWorker_SubscribeJobProgressClient, error)
}

// APIWorkerServer defines server RPC methods implemented by the worker service.
type APIWorkerServer interface {
	GetJobStatus(context.Context, *GetJobStatusRequest) (*JobStatusReply, error)
	SubscribeJobProgress(*SubscribeJobProgressRequest, APIWorker_SubscribeJobProgressServer) error
}

// APIWorker_SubscribeJobProgressClient provides stream receive helpers for clients.
type APIWorker_SubscribeJobProgressClient interface {
	Recv() (*JobStatusReply, error)
	grpc.ClientStream
}

// APIWorker_SubscribeJobProgressServer provides stream send helpers for servers.
type APIWorker_SubscribeJobProgressServer interface {
	Send(*JobStatusReply) error
	grpc.ServerStream
}

// UnimplementedAPIWorkerServer provides forward-compatible default server behavior.
type UnimplementedAPIWorkerServer struct{}

// GetJobStatus returns an unimplemented error by default.
func (UnimplementedAPIWorkerServer) GetJobStatus(context.Context, *GetJobStatusRequest) (*JobStatusReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetJobStatus not implemented")
}

// SubscribeJobProgress returns an unimplemented error by default.
func (UnimplementedAPIWorkerServer) SubscribeJobProgress(*SubscribeJobProgressRequest, APIWorker_SubscribeJobProgressServer) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeJobProgress not implemented")
}

// NewAPIWorkerClient builds a typed client around a grpc.ClientConn.
func NewAPIWorkerClient(cc grpc.ClientConnInterface) APIWorkerClient {
	return &apiWorkerClient{cc: cc}
}

type apiWorkerClient struct {
	cc grpc.ClientConnInterface
}

// GetJobStatus invokes unary worker status RPC.
func (c *apiWorkerClient) GetJobStatus(ctx context.Context, in *GetJobStatusRequest, opts ...grpc.CallOption) (*JobStatusReply, error) {
	out := new(JobStatusReply)
	err := c.cc.Invoke(ctx, "/"+ServiceName+"/GetJobStatus", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SubscribeJobProgress invokes streaming worker progress RPC.
func (c *apiWorkerClient) SubscribeJobProgress(ctx context.Context, in *SubscribeJobProgressRequest, opts ...grpc.CallOption) (APIWorker_SubscribeJobProgressClient, error) {
	stream, err := c.cc.NewStream(ctx, &APIWorker_ServiceDesc.Streams[0], "/"+ServiceName+"/SubscribeJobProgress", opts...)
	if err != nil {
		return nil, err
	}
	x := &apiWorkerSubscribeClient{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type apiWorkerSubscribeClient struct {
	grpc.ClientStream
}

// Recv receives one JobStatusReply from server stream.
func (x *apiWorkerSubscribeClient) Recv() (*JobStatusReply, error) {
	m := new(JobStatusReply)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// RegisterAPIWorkerServer binds the APIWorker service implementation to grpc.Server.
func RegisterAPIWorkerServer(s grpc.ServiceRegistrar, srv APIWorkerServer) {
	s.RegisterService(&APIWorker_ServiceDesc, srv)
}

// APIWorker_ServiceDesc describes service methods/streams for manual registration.
var APIWorker_ServiceDesc = grpc.ServiceDesc{
	ServiceName: ServiceName,
	HandlerType: (*APIWorkerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetJobStatus",
			Handler:    _APIWorker_GetJobStatus_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SubscribeJobProgress",
			Handler:       _APIWorker_SubscribeJobProgress_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "contracts/grpc/api-worker-v1.proto",
}

// _APIWorker_GetJobStatus_Handler dispatches unary GetJobStatus RPC calls.
func _APIWorker_GetJobStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetJobStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(APIWorkerServer).GetJobStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/" + ServiceName + "/GetJobStatus",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(APIWorkerServer).GetJobStatus(ctx, req.(*GetJobStatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// _APIWorker_SubscribeJobProgress_Handler dispatches streaming RPC calls.
func _APIWorker_SubscribeJobProgress_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(SubscribeJobProgressRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(APIWorkerServer).SubscribeJobProgress(m, &apiWorkerSubscribeServer{ServerStream: stream})
}

type apiWorkerSubscribeServer struct {
	grpc.ServerStream
}

// Send writes one stream event back to the caller.
func (x *apiWorkerSubscribeServer) Send(m *JobStatusReply) error {
	return x.ServerStream.SendMsg(m)
}

// DefaultClientCallOptions returns required call options for JSON gRPC payloads.
func DefaultClientCallOptions() []grpc.CallOption {
	return []grpc.CallOption{
		grpc.CallContentSubtype(jsonCodecName),
		grpc.ForceCodec(jsonCodec{}),
	}
}

// init registers JSON codec so grpc can marshal/unmarshal non-protobuf payloads.
func init() {
	encoding.RegisterCodec(jsonCodec{})
}

// jsonCodec marshals gRPC request/response payloads with standard JSON encoding.
type jsonCodec struct{}

// Name returns the registered codec name.
func (jsonCodec) Name() string {
	return jsonCodecName
}

// Marshal encodes one payload value.
func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal decodes one payload value.
func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(v); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return err
	}
	return nil
}
