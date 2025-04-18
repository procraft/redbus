// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.3
// source: api.proto

package pb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	RedbusService_Produce_FullMethodName = "/sergiusd.redbus.RedbusService/Produce"
	RedbusService_Consume_FullMethodName = "/sergiusd.redbus.RedbusService/Consume"
)

// RedbusServiceClient is the client API for RedbusService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type RedbusServiceClient interface {
	Produce(ctx context.Context, in *ProduceRequest, opts ...grpc.CallOption) (*ProduceResponse, error)
	Consume(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[ConsumeRequest, ConsumeResponse], error)
}

type redbusServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewRedbusServiceClient(cc grpc.ClientConnInterface) RedbusServiceClient {
	return &redbusServiceClient{cc}
}

func (c *redbusServiceClient) Produce(ctx context.Context, in *ProduceRequest, opts ...grpc.CallOption) (*ProduceResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ProduceResponse)
	err := c.cc.Invoke(ctx, RedbusService_Produce_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *redbusServiceClient) Consume(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[ConsumeRequest, ConsumeResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &RedbusService_ServiceDesc.Streams[0], RedbusService_Consume_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[ConsumeRequest, ConsumeResponse]{ClientStream: stream}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type RedbusService_ConsumeClient = grpc.BidiStreamingClient[ConsumeRequest, ConsumeResponse]

// RedbusServiceServer is the server API for RedbusService service.
// All implementations should embed UnimplementedRedbusServiceServer
// for forward compatibility.
type RedbusServiceServer interface {
	Produce(context.Context, *ProduceRequest) (*ProduceResponse, error)
	Consume(grpc.BidiStreamingServer[ConsumeRequest, ConsumeResponse]) error
}

// UnimplementedRedbusServiceServer should be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedRedbusServiceServer struct{}

func (UnimplementedRedbusServiceServer) Produce(context.Context, *ProduceRequest) (*ProduceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Produce not implemented")
}
func (UnimplementedRedbusServiceServer) Consume(grpc.BidiStreamingServer[ConsumeRequest, ConsumeResponse]) error {
	return status.Errorf(codes.Unimplemented, "method Consume not implemented")
}
func (UnimplementedRedbusServiceServer) testEmbeddedByValue() {}

// UnsafeRedbusServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to RedbusServiceServer will
// result in compilation errors.
type UnsafeRedbusServiceServer interface {
	mustEmbedUnimplementedRedbusServiceServer()
}

func RegisterRedbusServiceServer(s grpc.ServiceRegistrar, srv RedbusServiceServer) {
	// If the following call pancis, it indicates UnimplementedRedbusServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&RedbusService_ServiceDesc, srv)
}

func _RedbusService_Produce_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProduceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RedbusServiceServer).Produce(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: RedbusService_Produce_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RedbusServiceServer).Produce(ctx, req.(*ProduceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _RedbusService_Consume_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(RedbusServiceServer).Consume(&grpc.GenericServerStream[ConsumeRequest, ConsumeResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type RedbusService_ConsumeServer = grpc.BidiStreamingServer[ConsumeRequest, ConsumeResponse]

// RedbusService_ServiceDesc is the grpc.ServiceDesc for RedbusService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var RedbusService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "sergiusd.redbus.RedbusService",
	HandlerType: (*RedbusServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Produce",
			Handler:    _RedbusService_Produce_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Consume",
			Handler:       _RedbusService_Consume_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "api.proto",
}
