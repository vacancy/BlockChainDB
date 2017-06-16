package main

import (
    "log"
    "net"

    "golang.org/x/net/context"
    "google.golang.org/grpc"
    "google.golang.org/grpc/reflection"
    pb "../protobuf/go"
)

type Server struct{
    Config *ServerConfig
    Master  MinerMaster
}

func NewServer(config *ServerConfig) (s *Server, err error) {
    s = &Server{Config: config}
    s.Master, err = NewMinerMaster(config)
    if err != nil {
        return
    }
    return
}

// Client-side database interface 
func (s *Server) Get(ctx context.Context, in *pb.GetRequest) (*pb.GetResponse, error) {
    return &pb.GetResponse{Value: 1000}, nil
}
func (s *Server) GetHeight(ctx context.Context, in *pb.Null) (*pb.GetHeightResponse, error) {
    return &pb.GetHeightResponse{Height: 1, LeafHash: "?"}, nil
}
func (s *Server) Verify(ctx context.Context, in *pb.Transaction) (*pb.VerifyResponse, error) {
    return &pb.VerifyResponse{Result: pb.VerifyResponse_FAILED, BlockHash:"?"}, nil
}
func (s *Server) Transfer(ctx context.Context, in *pb.Transaction) (*pb.BooleanResponse, error) {
    return &pb.BooleanResponse{Success: true}, nil
}

func (s *Server) GetBlock(ctx context.Context, in *pb.GetBlockRequest) (*pb.JsonBlockString, error) {
    return &pb.JsonBlockString{Json: "{}"}, nil
}
func (s *Server) PushBlock(ctx context.Context, in *pb.JsonBlockString) (*pb.Null, error) {
    return &pb.Null{}, nil
}
func (s *Server) PushTransaction(ctx context.Context, in *pb.Transaction) (*pb.Null, error) {
    return &pb.Null{}, nil
}

func (s *Server) Mainloop() (err error) {
    // Should start master here

    // Bind to port
    lis, err := net.Listen("tcp", s.Config.Self.Addr)
    if err != nil {
        log.Printf("Failed to listen: %v", err)
        return
    }
    log.Printf("Listening: %s ...", s.Config.Self.Addr)

    // Create gRPC server
    rpc := grpc.NewServer()
    pb.RegisterBlockChainMinerServer(rpc, s)
    // Register reflection service on gRPC server.
    reflection.Register(rpc)

    // Start server
    if err = rpc.Serve(lis); err != nil {
        log.Printf("Failed to serve: %v", err)
        return
    }

    return
}

