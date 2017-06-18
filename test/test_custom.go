package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "time"
    "io/ioutil"
    "log"
    "math/rand"
    "strings"

    "../hash"
    pb "../protobuf/go"

    "golang.org/x/net/context"
    "google.golang.org/grpc"
    "github.com/golang/protobuf/jsonpb"
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

func makeTrans(FromID string, ToID string, Value int, Fee int) (t *pb.Transaction) {
    return &pb.Transaction{
            Type:pb.Transaction_TRANSFER,
            UUID:UUID128bit(),
            FromID: FromID, ToID: ToID, Value: int32(Value), MiningFee: int32(Fee)}
}

func makeBlock(blockID int, prevHash string, trans []*pb.Transaction, minerID string) (block *pb.Block, jsonString string, hashRes string, err error){
    block = &pb.Block{
        BlockID: int32(blockID),
        PrevHash: prevHash,
        Transactions: trans,
        MinerID: minerID,
        Nonce: "00000000",
    }
    jsonMarshaler := &jsonpb.Marshaler{EnumsAsInts: false}
    json, err := jsonMarshaler.MarshalToString(block)
    if err != nil {
        return
    }
    presuf := strings.Split(json, "\"Nonce\":\"00000000\"")
    if len(presuf) != 2 {
        err = fmt.Errorf("GG")
        return
    }
    prefix, suffix := presuf[0], presuf[1]
    prefix = prefix + "\"Nonce\":\""
    suffix = "\"" + suffix

    for i := 0; true; i++ {
        // nonce := fmt.Sprintf("%08x", w.rng.Uint32())
        nonce := fmt.Sprintf("%08x", i)

        str := strings.Join([]string{prefix, nonce, suffix}, "")
        hashRes = hash.GetHashString(str)
        succ := hash.CheckHash(hashRes)

        if succ {
            block.Nonce = nonce
            jsonString = str
            break
        }
    }
    return
}

func Transfer(c pb.BlockChainMinerClient, FromID string, ToID string, Value int, Fee int) (trans *pb.Transaction, succ bool, err error) {
    log.Printf("[TRANSFER] %s -> %s, Value: %d, MiningFee: %d", FromID, ToID, Value, Fee)
    trans = makeTrans(FromID, ToID, Value, Fee)
    r, err := c.Transfer(context.Background(), trans)
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

func Verify(c pb.BlockChainMinerClient, trans *pb.Transaction) (res int, err error) {
    log.Printf("[VERIFY] %v", trans)
    r, err := c.Verify(context.Background(), trans)
    if err != nil {
        log.Printf("VERIFY Error: %v", err)
        return
    } else {
        res = int(r.Result)
        log.Printf("VERIFY Return: %d", res)
        log.Printf("VERIFY Return Block: %s", r.BlockHash)
    }
    return
}

func PushTransaction(c pb.BlockChainMinerClient, trans *pb.Transaction) (err error) {
    log.Printf("[PUSH TRANSACTION] %v", trans)
    _, err = c.PushTransaction(context.Background(), trans)
    if err != nil {
        log.Printf("PUSH TRANSACTION Error: %v", err)
        return
    }
    return
}

func PushBlock(c pb.BlockChainMinerClient, json string) (err error) {
    log.Printf("[PUSH BLOCK] %s", json)
    _, err = c.PushBlock(context.Background(), &pb.JsonBlockString{Json: json})
    if err != nil {
        log.Printf("PUSH TRANSACTION Error: %v", err)
        return
    }
    return
}

func GetBlock(c pb.BlockChainMinerClient, blockHash string) (json string, err error) {
    log.Printf("[GET BLOCK] %s", blockHash)
    res, err := c.GetBlock(context.Background(), &pb.GetBlockRequest{BlockHash: blockHash})
    if err != nil {
        log.Printf("GET BLOCK Error: %v", err)
        return
    } else {
        json = res.Json
        log.Printf("GET BLOCK return: %s", json)
        return
    }
    return
}

func doTest(c pb.BlockChainMinerClient, cur int) (passed bool, err error) {
    passed = false
    succ := false
    res := 0

    //NOTE: This time should be adjust according to hardness, wait for the blocks to be computed.
    sleepForBlock := 5000 * time.Millisecond 

    switch cur {

    case 0:
        log.Printf("Check verify transaction (FAIL if invalid on longest branch)")
        t1 := makeTrans("T00U0000", "T00U0001", 1000, 1)
        t2 := makeTrans("T00U0000", "T00U0001", 8, 2)
        t3 := makeTrans("T00U0000", "T00U0001", 5, 3)
        t4 := makeTrans("T00U0001", "T00U0000", 3, 1)
        trans2 := []*pb.Transaction{t2}
        trans3 := []*pb.Transaction{t3}

        root := strings.Repeat("0", 64)
        blockID := 0
        _, json, hashRes, err := makeBlock(blockID + 1, root, trans2, "Server01")
        if err != nil {
            return false, err
        }
        log.Printf("hash: %s", hashRes)
        blockID += 1
        root = hashRes

        queryHash := hashRes

        json3 := json
        _, json3, hashRes, err = makeBlock(blockID + 1, root, trans3, "Server02")
        if err != nil {
            return false, err
        }
        log.Printf("hash: %s", hashRes)
        blockID += 1
        root = hashRes

        err = PushTransaction(c, t1)
        if err != nil {
            return false, err
        }
        err = PushBlock(c, json)
        if err != nil {
            return false, err
        }
        err = PushBlock(c, json3)
        if err != nil {
            return false, err
        }

        time.Sleep(sleepForBlock)

        res, err = Verify(c, t1)
        if err != nil {
            return false, err
        }
        // NOTE: 2 blocks pushed after transaction pushed. Thus become invalid on longest branch.
        if res != 0 {
            log.Printf("incorrect VERIFY result, result: %d, expected: %d", res, 0)
            return false, nil
        }

        // And the block should exist.
        ret, err := GetBlock(c, queryHash)
        if err != nil {
            return false, err
        }
        log.Printf("Expected return: %s", json)
        if ret != json {
            log.Printf("incorrect GET BLOCK result, result: %s, expected: %s", ret, json)
            return false, nil
        }

        err = PushTransaction(c, t4)
        if err != nil {
            return false, err
        }

    case 1:
        log.Printf("Check transfer (Value <= MiningFee)")
        _, succ, err = Transfer(c, "T01U0000", "T01U0001", 5, 5)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }
        _, succ, err = Transfer(c, "T01U0000", "T01U0001", 30, 50)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }

    case 2:
        log.Printf("Check transfer (MiningFee <= 0)")
        _, succ, err = Transfer(c, "T02U0000", "T02U0001", 10, -1)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }
        _, succ, err = Transfer(c, "T02U0000", "T02U0001", 100, 0)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }

    case 3:
        log.Printf("Check transfer (Value < 0)")
        _, succ, err = Transfer(c, "T03U0000", "T03U0001", -2, 5)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }
    
    case 4:
        log.Printf("Check transfer (FAKE User ID)")
        _, succ, err = Transfer(c, "FAKE00", "T04U0001", 10, 5)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }
        _, succ, err = Transfer(c, "T04U0000", "FAKE000001", 10, 5)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }

    case 5:
        log.Printf("Check transfer (FromID == ToID)")
        _, succ, err = Transfer(c, "T05U0000", "T05U0000", 5, 1)
        log.Printf("Expected return: false")
        if err != nil || succ {
            return
        }

    case 6: 
        log.Printf("Check transfer (multi -> one)")
        for i := 0; i < 10; i ++ {
            FromID := fmt.Sprintf("T06U00%02d", i)
            _, succ, err = Transfer(c, FromID, "T06U9999", 10, 5)
            log.Printf("Expected return: true")
            if err != nil || !succ {
                return
            }
        }

        time.Sleep(sleepForBlock)
        res, err = Get(c, "T06U9999")
        if err != nil {
            return
        }
        expected := 1050
        log.Printf("Expected return: %d", expected)
        if res != expected {
            log.Printf("incorrect GET result, result: %d, expected: %d", res, expected)
            return
        }

        res, err = Get(c, "T06U0005")
        if err != nil {
            return
        }
        expected = 990
        log.Printf("Expected return: %d", expected)
        if res != expected {
            log.Printf("incorrect GET result, result: %d, expected: %d", res, expected)
            return
        }

    case 7: 
        log.Printf("Check transfer (make impossible trans possible)")
        _, succ, err = Transfer(c, "T07U0000", "T07U9999", 10, 5)
        log.Printf("Expected return: true")
        if err != nil || !succ {
            return
        }
        expected0 := 990
        expected9 := 1005
        _, succ, err = Transfer(c, "T07U9999", "T07U0000", 2000, 1)
        if err != nil {
            return
        }
        if succ {
            expected0 += 1999
            expected9 -= 2000
        }
        _, succ, err = Transfer(c, "T07U0000", "T07U9999", 1000, 1)
        if err != nil {
            return
        }
        if succ && expected0 >= 1000{
            expected0 -= 1000
            expected9 += 999
        }
        _, succ, err = Transfer(c, "T07U5555", "T07U9999", 1000, 1)
        if err != nil || !succ {
            return
        }
        expected9 += 999

        time.Sleep(sleepForBlock)
        res, err = Get(c, "T07U0000")
        if err != nil {
            return
        }
        expected := expected0
        log.Printf("Expected return: %d", expected)
        if res != expected {
            log.Printf("incorrect GET result, result: %d, expected: %d", res, expected)
            return
        }

        res, err = Get(c, "T07U9999")
        if err != nil {
            return
        }
        expected = expected9
        log.Printf("Expected return: %d", expected)
        if res != expected {
            log.Printf("incorrect GET result, result: %d, expected: %d", res, expected)
            return
        }

    case 8:
        log.Printf("Check transfer (collect money)")

        nUser := 30
        // money: expected money for each user
        var money []int
        // received: whether server accept these transactions
        var received []bool
        var trans []*pb.Transaction
        var t *pb.Transaction

        for i := 0; i <= nUser; i ++ {
            money = append(money, 1000)
        }
        for i := 0; i < nUser; i ++ {
            FromID := fmt.Sprintf("T08U%04d", i)
            ToID := fmt.Sprintf("T08U%04d", i + 1)
            amount := 1000 + i * (1000 - 1)
            // [TRANSFER] User i give all his money to User i + 1
            t, succ, err = Transfer(c, FromID, ToID, amount, 1)
            log.Printf("Expected return: true")
            if err != nil {
                return
            }
            if succ && money[i] >= amount {
                money[i] -= amount
                money[i + 1] += amount - 1
            }
            received = append(received, succ)
            trans = append(trans, t)
        }

        for i := 0; i < 4; i ++ {
            time.Sleep(sleepForBlock)
        }

        expected := 2
        for i := 0; i < nUser; i ++ {
            res, err = Verify(c, trans[i])
            if err != nil {
                return
            }
            if res < expected {
                expected = res
            }
            if received[i] && (res == 0 && res != expected) || (res > expected){
                log.Printf("incorrect VERIFY result, result: %d, expected: %d", res, expected)
                return
            }
            if !received[i] && res != 0 {
                log.Printf("incorrect VERIFY result, result: %d, expected: %d", res, 0)
                return
            }
        }

        // test fake transaction, verify using same UUID, but different content
        fakeTrans := trans[0]
        fakeTrans.Value = 998
        res, err = Verify(c, trans[0])
        if err != nil {
            return
        }
        if res != 0 {
            log.Printf("transaction has been manipulated, need check within same UUID")
            log.Printf("incorrect VERIFY result, result: %d, expected: %d", res, expected)
            return
        }

        for i := 0; i <= nUser; i ++ {
            UserID := fmt.Sprintf("T08U%04d", i)
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

    case 9:
        log.Printf("Check block (incorrect BlockID)")
        t := makeTrans("T09U0000", "T09U0001", 5, 1)
        trans := []*pb.Transaction{t}

        root := strings.Repeat("0", 64)
        blockID := 0
        _, json, hashRes, err := makeBlock(blockID + 2, root, trans, "Server01")
        if err != nil {
            return false, err
        }
        log.Printf("hash: %s", hashRes)
        blockID += 1
        root = hashRes
        err = PushBlock(c, json)
        
        time.Sleep(sleepForBlock)

        ret, err := GetBlock(c, hashRes)
        if err != nil {
            return false, err
        }
        log.Printf("Expected return: ")
        if ret == json {
            log.Printf("incorrect GET BLOCK result, result: %s, expected: %s", ret, "")
            return false, nil
        }

    case 10:
        log.Printf("Check block (duplicate trans in block)")
        t := makeTrans("T10U0000", "T10U0001", 5, 1)
        trans := []*pb.Transaction{t, t}

        root := strings.Repeat("0", 64)
        blockID := 0
        _, json, hashRes, err := makeBlock(blockID + 1, root, trans, "Server01")
        if err != nil {
            return false, err
        }
        log.Printf("hash: %s", hashRes)
        blockID += 1
        root = hashRes
        err = PushBlock(c, json)
        if err != nil {
            return false, err
        }
        
        time.Sleep(sleepForBlock)

        ret, err := GetBlock(c, hashRes)
        if err != nil {
            return false, err
        }
        log.Printf("Expected return: ")
        if ret == json {
            log.Printf("incorrect GET BLOCK result, result: %s, expected: %s", ret, "")
            return false, nil
        }

    case 11:
        log.Printf("Check block (incorrect MinerID)")
        t := makeTrans("T11U0000", "T11U0001", 5, 1)
        trans := []*pb.Transaction{t}

        root := strings.Repeat("0", 64)
        blockID := 0
        _, json, hashRes, err := makeBlock(blockID + 1, root, trans, "Serveroo")
        if err != nil {
            return false, err
        }
        log.Printf("hash: %s", hashRes)
        blockID += 1
        root = hashRes
        err = PushBlock(c, json)
        if err != nil {
            return false, err
        }

        time.Sleep(sleepForBlock)

        ret, err := GetBlock(c, hashRes)
        if err != nil {
            return false, err
        }
        log.Printf("Expected return: ")
        if ret == json {
            log.Printf("incorrect GET BLOCK result, result: %s, expected: %s", ret, "")
            return false, nil
        }

    // For self-test, might take some time. directly use args: -T 12
    case 12:
        log.Printf("Check GET (Miner money)")
        serverID := "Server01"
        res, err = Get(c, serverID)
        if err != nil {
            return
        }
        balance := res

        var trans []*pb.Transaction
        var t *pb.Transaction

        for i := 0; i < 1000; i ++ {
            x := rand.Intn(1000)
            y := rand.Intn(1000)
            for x == y {
                y = rand.Intn(1000)
            }
            v := rand.Intn(20) + 2
            fee := rand.Intn(v - 1) + 1
            FromID := fmt.Sprintf("T12U%04d", x)
            ToID := fmt.Sprintf("T12U%04d", y)
            t, succ, err = Transfer(c, FromID, ToID, v, fee)
            if err != nil {
                return
            }
            trans = append(trans, t)

            if i % 100 == 0 {
                for j := 0; j < 3; j ++ {
                    time.Sleep(sleepForBlock)
                    res, err = Get(c, serverID)
                    if err != nil {
                        return
                    }
                    log.Printf("Miner %s get money %d", serverID, res - balance)
                }
            }
        }

        // TODO: get blockHash by Verify, get Block.MinerID by GetBlock.
        // for i = 0; i < 100000; i ++ {
        //     res, err = Verify(c, trans[i])
        // }

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

    nTests := 12
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
