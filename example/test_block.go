package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "time"
    "io/ioutil"
    "log"
    "math/rand"

    pb "../protobuf/go"

    "golang.org/x/net/context"
    "google.golang.org/grpc"
)

var (
    ServerID = flag.String("server", "2", "Server ID.")
    Hash = flag.String("hash", "", "Hash.")
)

func UUID128bit() string {
    // Returns a 128bit hex string, RFC4122-compliant UUIDv4
    u:=make([]byte,16)
    _,_=rand.Read(u)
    // this make sure that the 13th character is "4"
    u[6] = (u[6] | 0x40) & 0x4F
    // this make sure that the 17th is "8", "9", "a", or "b"
    u[8] = (u[8] | 0x80) & 0xBF 
    return fmt.Sprintf("%x",u)
}

func main() {
    flag.Parse()
    rand.Seed(int64(time.Now().Nanosecond()))

    var address = func() string {
        conf, err := ioutil.ReadFile("config.json")
        if err != nil {
            panic(err)
        }
        var dat map[string]interface{}
        err = json.Unmarshal(conf, &dat)
        if err != nil {
            panic(err)
        }
        dat = dat[*ServerID].(map[string]interface{})
        return fmt.Sprintf("%s:%s", dat["ip"], dat["port"])
    }()

    // Set up a connection to the server.
    conn, err := grpc.Dial(address, grpc.WithInsecure())
    if err != nil {
        log.Fatalf("Cannot connect to server: %v", err)
    }
    defer conn.Close()
    c := pb.NewBlockChainMinerClient(conn)

    if r, err := c.GetBlock(context.Background(), &pb.GetBlockRequest{BlockHash: *Hash}); err != nil {
        log.Printf("GET Error: %v", err)
    } else {
        log.Printf("GET Return: %s", r.Json)
    }
}
