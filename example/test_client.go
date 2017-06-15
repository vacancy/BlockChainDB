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
    dat = dat["1"].(map[string]interface{})
    return fmt.Sprintf("%s:%s", dat["ip"], dat["port"])
}()

var (
    OpCode = flag.String("T", "", `DB transactions to perform:
    1 (or GET):      Show the balance of a given UserID. 
        Require option -user.
    5 (or TRANSFER): Transfer some money from one account to another.
        Require option -from, -to, -value.
    // 6 (or VERIFY): ...
    `)
    UserID = flag.String("user", "00000000", "User account ID for the operation.")
    FromID = flag.String("from", "00000000", "From account (for Transfer)")
    ToID   = flag.String("to", "12345678", "To account (for Transfer)")
    Value  = flag.Int("value", 1, "Amount of transaction")
    Fee  = flag.Int("fee", 1, "Mining Fee of transaction")
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
    fmt.Println(UUID128bit())

    // Set up a connection to the server.
    conn, err := grpc.Dial(address, grpc.WithInsecure())
    if err != nil {
        log.Fatalf("Cannot connect to server: %v", err)
    }
    defer conn.Close()
    //new client
    c := pb.NewBlockChainMinerClient(conn)

    switch *OpCode {
    case "":
        log.Fatal("Please specify an operation to perform (-T [1-5]). Run with -help to see a list of supported operations.")
    default:
        log.Fatal("Unknown operation.")
    case "1", "GET":
        if r, err := c.Get(context.Background(), &pb.GetRequest{UserID: *UserID}); err != nil {
            log.Printf("GET Error: %v", err)
        } else {
            log.Printf("GET Return: %d", r.Value)
        }
    case "5", "TRANSFER":
        if r, err := c.Transfer(context.Background(), &pb.Transaction{
                Type:pb.Transaction_TRANSFER,
                UUID:UUID128bit(),
                FromID: *FromID, ToID: *ToID, Value: int32(*Value), MiningFee: int32(*Fee)}); err != nil {
            log.Printf("TRANSFER Error: %v", err)
        } else {
            log.Printf("TRANSFER Return: %s", r)
        }
    //case Verify, GetBlock, GetHeight omitted.
    }
}
