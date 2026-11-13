package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/reflection"

	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/grpc-ecosystem/go-grpc-middleware/tags"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/Avielyo10/goflake/config"
	"github.com/Avielyo10/goflake/internal/app"
	pb "github.com/Avielyo10/goflake/internal/common/genproto/api/protobuf"
)

// maxGRPCRecvMsgSize is the largest request the grpc server accepts, in bytes
const maxGRPCRecvMsgSize = 4 << 10

type GRPCServer struct {
	pb.UnimplementedFlakeServiceServer
	config  *config.Config
	flacker *app.Flacker
	log     *logrus.Logger
}

// NewGRPCServer creates a new grpc server
func NewGRPCServer(cfg *config.Config, log *logrus.Logger) *GRPCServer {
	return &GRPCServer{config: cfg, flacker: app.NewFlacker(*cfg), log: log}
}

// Serve starts the grpc server, implements the Server interface
func (s *GRPCServer) Serve() error {
	// Create a listener on TCP port
	lis, err := net.Listen("tcp", s.config.Server.Addr())
	if err != nil {
		return err
	}
	// requests are a few bytes long, there is no reason to buffer the default 4MB for anyone
	opts := []grpc.ServerOption{grpc.MaxRecvMsgSize(maxGRPCRecvMsgSize)}
	// Set TLS credentials if enabled
	if s.config.Server.TLS.CertPath != "" && s.config.Server.TLS.KeyPath != "" {
		creds, err := credentials.NewServerTLSFromFile(s.config.Server.TLS.CertPath, s.config.Server.TLS.KeyPath)
		if err != nil {
			return err
		}
		opts = append(opts, grpc.Creds(creds))
	}
	// Set the logger as a server middleware
	logrusOpts := []grpc_logrus.Option{
		grpc_logrus.WithDurationField(func(duration time.Duration) (key string, value interface{}) {
			return "grpc.time_ns", duration.Nanoseconds()
		}),
	}
	// log through the configured logger so that log_level applies to request logs as well
	logrusEntry := logrus.NewEntry(s.log)
	opts = append(
		opts,
		grpc_middleware.WithUnaryServerChain(
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_logrus.UnaryServerInterceptor(logrusEntry, logrusOpts...),
			// a panicking handler fails its own request instead of taking the server down
			grpc_recovery.UnaryServerInterceptor(),
		),
	)
	// Create a server
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterFlakeServiceServer(grpcServer, s)
	// Register reflection service on grpc server
	if s.config.Env == config.DevelopmentEnvType {
		reflection.Register(grpcServer)
	}
	return grpcServer.Serve(lis)
}

// GetUUID implements the FlakeServiceServer interface
func (s *GRPCServer) GetUUID(ctx context.Context, req *pb.GetUUIDRequest) (*pb.GetUUIDResponse, error) {
	s.log.Debug("Generating new uuid")
	uuid, err := s.flacker.NextUUID()
	if err != nil {
		s.log.Error("Failed to generate uuid: ", err)
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &pb.GetUUIDResponse{Uuid: uuid}, nil
}

// Decompose implements the FlakeServiceServer interface
func (s *GRPCServer) Decompose(ctx context.Context, req *pb.DecomposeRequest) (*pb.DecomposeResponse, error) {
	uuid := req.GetUuid()
	s.log.Debug("Decomposing: ", uuid)
	decomposed := s.flacker.Decompose(uuid)
	timestamp, ok := s.flacker.Time(decomposed["time"])
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "uuid timestamp is out of range")
	}
	return &pb.DecomposeResponse{
		Uuid:         fmt.Sprint(uuid),
		Timestamp:    timestamp.Format(time.RFC3339Nano),
		DatacenterId: fmt.Sprint(decomposed["datacenter_id"]),
		MachineId:    fmt.Sprint(decomposed["machine_id"]),
		Sequence:     fmt.Sprint(decomposed["sequence"]),
		Msb:          decomposed["msb"] == 1,
	}, nil
}
