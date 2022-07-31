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
	"github.com/grpc-ecosystem/go-grpc-middleware/tags"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/Avielyo10/goflake/config"
	"github.com/Avielyo10/goflake/internal/app"
	pb "github.com/Avielyo10/goflake/internal/common/genproto/api/protobuf"
)

type GRPCServer struct {
	pb.UnimplementedFlakeServiceServer
	Config  *config.Config
	flacker *app.Flacker
	log     *logrus.Logger
}

// Serve starts the grpc server, implements the Server interface
func (s *GRPCServer) Serve() error {
	// Create a listener on TCP port
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.Config.Server.Host, s.Config.Server.Port))
	if err != nil {
		return err
	}
	var opts []grpc.ServerOption
	// Set TLS credentials if enabled
	if s.Config.Server.TLS.Enabled {
		creds, err := credentials.NewServerTLSFromFile(s.Config.Server.TLS.CertPath, s.Config.Server.TLS.KeyPath)
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
	logrusEntry := logrus.NewEntry(logrus.StandardLogger())
	opts = append(
		opts,
		grpc_middleware.WithUnaryServerChain(
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_logrus.UnaryServerInterceptor(logrusEntry, logrusOpts...),
		),
	)
	// Create a server
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterFlakeServiceServer(grpcServer, s)
	// Register reflection service on grpc server
	if s.Config.Env == config.DevelopmentEnvType {
		reflection.Register(grpcServer)
	}
	return grpcServer.Serve(lis)
}

// GetUUID implements the FlakeServiceServer interface
func (s *GRPCServer) GetUUID(ctx context.Context, req *pb.GetUUIDRequest) (*pb.GetUUIDResponse, error) {
	s.log.Debug("Generating new uuid")
	return &pb.GetUUIDResponse{Uuid: s.flacker.NextUUID()}, nil
}

// Decompose implements the FlakeServiceServer interface
func (s *GRPCServer) Decompose(ctx context.Context, req *pb.DecomposeRequest) (*pb.DecomposeResponse, error) {
	s.log.Debug("Decomposing: ", req.Uuid)
	decomposed := s.flacker.Decompose(req.Uuid)
	msb := false
	if decomposed["msb"] == 1 {
		msb = true
	}
	return &pb.DecomposeResponse{
		Uuid:         fmt.Sprint(req.Uuid),
		Timestamp:    fmt.Sprint(decomposed["time"]),
		DatacenterId: fmt.Sprint(decomposed["datacenter_id"]),
		MachineId:    fmt.Sprint(decomposed["machine_id"]),
		Sequence:     fmt.Sprint(decomposed["sequence"]),
		Msb:          msb,
	}, nil
}
