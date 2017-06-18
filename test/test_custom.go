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
    TestCase = flag.Int("T", -1, "Test case no.")
)

func UUID128bit() string {
    // Returns a 128bit hex string, RFC4122-compliant UUIDv4
    u := make([]byte, 16)
    _, _ = rand.Read(u)
    // this make sure that the 13th character is "4"
    u[6] = (u[6] | 0x40) & 0x4F
    // this make sure that the 17th is "8", "9", "a", or "b"
    u[8] = (u[8] | 0x80) & 0xBF 
    return fmt.Sprintf("%x", u)
}

func Transfer(c pb.BlockChainMinerClient, FromID string, ToID string, Value int, Fee int) (succ bool, err error){
    log.Printf("[TRANSFER] %s -> %s, Value: %d, MiningFee: %d", FromID, ToID, Value, Fee)
    r, err := c.Transfer(context.Background(), &pb.Transaction{
            Type:pb.Transaction_TRANSFER,
            UUID:UUID128bit(),
            FromID: FromID, ToID: ToID, Value: int32(Value), MiningFee: int32(Fee)})
    if err != nil {
        log.Printf("TRANSFER Error: %v", err)
        return
    } else {
        log.Printf("TRANSFER Return: %v", r.Success)
        succ = r.Success
    }
    return
}

func Get(c pb.BlockChainMinerClient, UserID string) (res int, err error) {
    log.Printf("[GET] %s", UserID)
    r, err := c.Get(context.Background(), &pb.GetRequest{UserID: UserID})
    if err != nil {
        log.Printf("GET Error: %v", err)
        return
    } else {
        res = int(r.Value)
        log.Printf("GET Return: %d", res)
    }
    return
}

func doTest(c pb.BlockChainMinerClient, cur int) (passed bool, err error) {
    passed = false
    succ := false
    res := 0

    sleepForBlock := 5000 * time.Millisecond

    switch cur {
    case 0:
        log.Printf("Check transfer (FromID == ToID)")
        succ, err = Transfer(c, "T01U0000", "T01U0000", 5, 1)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }

    case 1:
        log.Printf("Check transfer (Value <= MiningFee)")
        succ, err = Transfer(c, "T02U0000", "T02U0001", 5, 5)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }
        succ, err = Transfer(c, "T02U0000", "T02U0001", 30, 50)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }

    case 2:
        log.Printf("Check transfer (MiningFee <= 0)")
        succ, err = Transfer(c, "T03U0000", "T03U0001", 10, -1)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }
        succ, err = Transfer(c, "T03U0000", "T03U0001", 100, 0)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }

    case 3:
        log.Printf("Check transfer (Value < 0)")
        succ, err = Transfer(c, "T03U0000", "T03U0001", -2, 5)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }
    
    case 4:
        log.Printf("Check transfer (FAKE User ID)")
        succ, err = Transfer(c, "FAKE00", "T04U0001", 10, 5)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }
        succ, err = Transfer(c, "T04U0000", "FAKE000001", 10, 5)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }

    case 5: 
        log.Printf("Check transfer (multi -> one)")
        for i := 0; i < 10; i ++ {
            FromID := fmt.Sprintf("T05U00%02d", i)
            succ, err = Transfer(c, FromID, "T05U9999", 10, 5)
            log.Printf("Expected return: true")
            if err != nil || !succ {
                return
            }
        }

        time.Sleep(sleepForBlock)
        res, err = Get(c, "T05U9999")
        if err != nil {
            return
        }
        expected := 1050
        log.Printf("Expected return: %d", expected)
        if res != expected {
            log.Printf("incorrect GET result, result: %d, expected: %d", res, expected)
            return
        }

        res, err = Get(c, "T05U0005")
        if err != nil {
            return
        }
        expected = 990
        log.Printf("Expected return: %d", expected)
        if res != expected {
            log.Printf("incorrect GET result, result: %d, expected: %d", res, expected)
            return
        }

    case 6: 
        log.Printf("Check transfer (valid later)")
        succ, err = Transfer(c, "T06U0000", "T06U9999", 10, 5)
        log.Printf("Expected return: true")
        if err != nil || !succ {
            return
        }
        expected0 := 990
        expected9 := 1005
        succ, err = Transfer(c, "T06U9999", "T06U0000", 2000, 1)
        if err != nil {
            return
        }
        if succ {
            expected0 += 1999
            expected9 -= 2000
        }
        succ, err = Transfer(c, "T06U0000", "T06U9999", 1000, 1)
        if err != nil {
            return
        }
        if succ && expected0 >= 1000{
            expected0 -= 1000
            expected9 += 999
        }
        succ, err = Transfer(c, "T06U5555", "T06U9999", 1000, 1)
        if err != nil || !succ {
            return
        }
        expected9 += 999

        time.Sleep(sleepForBlock)
        res, err = Get(c, "T06U0000")
        if err != nil {
            return
        }
        expected := expected0
        log.Printf("Expected return: %d", expected)
        if res != expected {
            log.Printf("incorrect GET result, result: %d, expected: %d", res, expected)
            return
        }

        res, err = Get(c, "T06U9999")
        if err != nil {
            return
        }
        expected = expected9
        log.Printf("Expected return: %d", expected)
        if res != expected {
            log.Printf("incorrect GET result, result: %d, expected: %d", res, expected)
            return
        }

    case 7:
        log.Printf("Check transfer (collect money)")

        nUser := 50
        var money []int

        for i := 0; i <= nUser; i ++ {
            money = append(money, 1000)
        }
        for i := 0; i < nUser; i ++ {
            FromID := fmt.Sprintf("T07U%04d", i)
            ToID := fmt.Sprintf("T07U%04d", i + 1)
            amount := 1000 + i * (1000 - 1)
            succ, err = Transfer(c, FromID, ToID, amount, 1)
            log.Printf("Expected return: true")
            if err != nil {
                return
            }
            if succ && money[i] >= amount {
                money[i] -= amount
                money[i + 1] += amount - 1
            }
        }

        for i := 0; i < 6; i ++ {
            time.Sleep(sleepForBlock)
        }

        for i := 0; i <= nUser; i ++ {
            UserID := fmt.Sprintf("T07U%04d", i)
            res, err = Get(c, UserID)
            if err != nil {
                return
            }
            expected := money[i]
            log.Printf("Expected return: %d", expected)
            if res != expected {
                log.Printf("incorrect GET result, result: %d, expected: %d", res, expected)
                return
            }
        }

    }
    passed = true
    return
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
    testNo := *TestCase

    nTests := 10
    nPassed := 0
    testAll := testNo < 0

    if testAll {
        log.Printf("Begin test, total %d tests", nTests)
    }

    for i := 0; i < nTests; i ++ {
        if testAll || testNo == i {
            log.Printf("----------------- Test %d -----------------", i)
            res, err := doTest(c, i)
            if err != nil {
                log.Printf("* Test %d failed *, got Error: %v", i, err)
            } else {
                if res {
                    nPassed += 1
                    log.Printf("* Test %d passed *", i)
                } else {
                    log.Printf("* Test %d failed *", i)
                }
            }
            log.Printf("=============== End Test %d ===============", i)
        }
    }

    if testAll {
        log.Printf("End test, %d tests passed, out of %d tests", nPassed, nTests)
    }

}
