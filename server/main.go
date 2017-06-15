package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net"
    "flag"

    pb "../protobuf/go"
    "../hash"

    "golang.org/x/net/context"
    "google.golang.org/grpc"
    "google.golang.org/grpc/reflection"
)

const maxBlockSize = 50

type server struct{}
// Database Interface 
func (s *server) Get(ctx context.Context, in *pb.GetRequest) (*pb.GetResponse, error) {
    return &pb.GetResponse{Value: 1000}, nil
}
func (s *server) Transfer(ctx context.Context, in *pb.Transaction) (*pb.BooleanResponse, error) {
    return &pb.BooleanResponse{Success: true}, nil
}
func (s *server) Verify(ctx context.Context, in *pb.Transaction) (*pb.VerifyResponse, error) {
    return &pb.VerifyResponse{Result: pb.VerifyResponse_FAILED, BlockHash:"?"}, nil
}

func (s *server) GetHeight(ctx context.Context, in *pb.Null) (*pb.GetHeightResponse, error) {
    return &pb.GetHeightResponse{Height: 1, LeafHash: "?"}, nil
}
func (s *server) GetBlock(ctx context.Context, in *pb.GetBlockRequest) (*pb.JsonBlockString, error) {
    return &pb.JsonBlockString{Json: "{}"}, nil
}
func (s *server) PushBlock(ctx context.Context, in *pb.JsonBlockString) (*pb.Null, error) {
    return &pb.Null{}, nil
}
func (s *server) PushTransaction(ctx context.Context, in *pb.Transaction) (*pb.Null, error) {
    return &pb.Null{}, nil
}

var id=flag.Int("id",1,"Server's ID, 1<=ID<=NServers")

// Main function, RPC server initialization
func main() {
    flag.Parse()
    IDstr:=fmt.Sprintf("%d",*id)

    _=fmt.Sprintf("Server%02d",*id) //MinerID=
    _=hash.GetHashString

    // Read config
    address, _ := func() (string, string) {
        conf, err := ioutil.ReadFile("config.json")
        if err != nil {
            panic(err)
        }
        var dat map[string]interface{}
        err = json.Unmarshal(conf, &dat)
        if err != nil {
            panic(err)
        }
        dat = dat[IDstr].(map[string]interface{}) // should be dat[myNum] in the future
        return fmt.Sprintf("%s:%s", dat["ip"], dat["port"]), fmt.Sprintf("%s",dat["dataDir"])
    }()

    // Bind to port
    lis, err := net.Listen("tcp", address)
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }
    log.Printf("Listening: %s ...", address)

    // Create gRPC server
    s := grpc.NewServer()
    pb.RegisterBlockChainMinerServer(s, &server{})
    // Register reflection service on gRPC server.
    reflection.Register(s)

    // Start server
    if err := s.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}
