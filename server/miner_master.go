package main

import (
    "fmt"
    "log"
    "time"
    "strings"
    "sync"
    "math/rand"

    pb "../protobuf/go"
    "github.com/golang/protobuf/jsonpb"
)

type OnReceiveResponse struct {
    c chan bool
}

func NewOnReceiveResponse() *OnReceiveResponse {
    return &OnReceiveResponse{
        c: make(chan bool, 1),
    }
}

func (r *OnReceiveResponse) Finish() {
    r.c <- true
}

func (r *OnReceiveResponse) Wait() bool {
    rc := <-r.c
    close(r.c)
    return rc
}

type MinerMaster interface {
    Recover() error
    Start()

    // Client-side
    GetUserInfo(uid string) *UserInfo
    GetLatestBlock() *BlockInfo
    // transaction => return code, blockhash
    VerifyClientTransaction(t *pb.Transaction) (int, string)
    OnClientTransactionAsync(t *pb.Transaction) bool

    // Peer-side
    GetBlock(bid string) *BlockInfo
    OnBlockAsync(json string)
    OnTransactionAsync(t *pb.Transaction)

    OnWorkerSuccess(json string, hash string)
}

func NewMinerMaster(c *ServerConfig) (m MinerMaster, e error) {
    p2pc := NewP2PClient(c)

    switch c.Miner.MinerType {
    case "Honest":
        m = &HonestMinerMaster{
            BaseMinerMaster: BaseMinerMaster{
                BC: NewBlockChain(c, p2pc),
                P2PC: p2pc,
                jsonMarshaler: &jsonpb.Marshaler{EnumsAsInts: false},
            },
            config: c,
            workers: make([]MinerWorker, 0),
            updateMutex: &sync.Mutex{},
            currentWorking: nil,
        }
    default:
        e = fmt.Errorf("Invalid miner type: %s", c.Miner.MinerType)
    }
    return
}

type BaseMinerMaster struct {
    BC  *BlockChain
    P2PC *P2PClient

    jsonMarshaler *jsonpb.Marshaler
}

func (m *BaseMinerMaster) GetUserInfo(uid string) *UserInfo {
    return m.BC.GetUserInfo(uid)
}

func (m *BaseMinerMaster) GetLatestBlock() *BlockInfo {
    return m.BC.GetLatestBlock()
}

func (m *BaseMinerMaster) VerifyClientTransaction(t *pb.Transaction) (rc int, hash string) {
    rc, hash = m.BC.VerifyTransaction6(t)
    return
}

type HonestMinerMaster struct {
    BaseMinerMaster

    config   *ServerConfig
    workers []MinerWorker
    updateMutex *sync.Mutex
    currentWorking *pb.Block
}

func (m *HonestMinerMaster) Recover() (err error) {
    // TODO::
    return nil
}

func (m *HonestMinerMaster) Start() {
    // Start workers
    totalRange := (1 << 32)
    rangeSize := int(totalRange / m.config.Miner.NrWorkers)

    for i := 0; i < m.config.Miner.NrWorkers; i++ {
        log.Printf("Starting worker #%d", i)
        w := NewSimpleMinerWorker(m, int64(rangeSize * i), int64(rangeSize * (i + 1)), m.config)
        m.workers = append(m.workers, w)
        go w.Mainloop()
    }
}

func (m *HonestMinerMaster) GetBlock(bid string) *BlockInfo {
    b, ok := m.BC.GetBlock(bid)
    if !ok || !b.OnLongest {
        return nil
    }
    return b
}

func (m *HonestMinerMaster) OnClientTransactionAsync(t *pb.Transaction) bool {
    m.updateMutex.Lock()
    defer m.updateMutex.Unlock()

    // Broadcast::
    if true {
        res := m.P2PC.RemotePushTransactionAsync(t)
        msg := res.Get()
        res.IgnoreLater()
        if msg == nil {
            return false
        }
    }

    return m.processTransaction(t)
}

func (m *HonestMinerMaster) OnTransactionAsync(t *pb.Transaction) {
    m.updateMutex.Lock()
    defer m.updateMutex.Unlock()

    _ = m.processTransaction(t)
}

func (m *HonestMinerMaster) OnBlockAsync(json string) {
    m.updateMutex.Lock()
    defer m.updateMutex.Unlock()

    lastChanged, _ := m.BC.PushBlockJson(json)
    if lastChanged {
        m.updateWorkingSet(true, false)
    }
}

func (m *HonestMinerMaster) OnWorkerSuccess(json string, hash string) {
    m.updateMutex.Lock()
    defer m.updateMutex.Unlock()

    _, err := m.BC.DeclareBlockJson(json)
    if err == nil {
        log.Printf("!! Mined: hash=%s.", hash)
        _ = m.P2PC.RemotePushBlockAsync(json)
        m.updateWorkingSet(true, false)
    } else {
        log.Printf("Got invalid declaration: %s, %v.", json, err)
    }
}

// TODO:: Whether to forward.
func (m *HonestMinerMaster) processTransaction(t *pb.Transaction) bool {
    // TODO:: Flow control

    ok := m.BC.PushTransaction(t, true)
    if !ok {
        return false
    }

    // Passed check
    m.updateWorkingSet(false, false)

    return true
}

func (m *HonestMinerMaster) updateWorkingSet(forceUpdate bool, allowSame bool) {
    // log.Printf("Updating working set invoked: ForceUpdate=%v AllowSame=%v NeedLock=%v.", forceUpdate, allowSame, needLock)

    if !forceUpdate {
        // Check non-working workers first
        flag := false
        for _, w := range m.workers {
            if !w.Working() {
                flag = true
                break
            }
        }

        if !flag {
            return
        }
    }

    if m.updateWorkingSetInternal(forceUpdate, allowSame) {
        return
    }

    if forceUpdate && m.config.Miner.EnableComputationIdle {
        // log.Printf("UpdateWorkingBlock: stopping workers.")
        for _, w := range m.workers {
            if w.Working() {
                w.UpdateWorkingBlock("", "")
            }
        }
    }
}

func (m *HonestMinerMaster) updateWorkingSetInternal(forceUpdate bool, allowSame bool) bool {
    // log.Printf("Updating working set internal invoked: ForceUpdate=%v AllowSame=%v NeedLock=%v.", forceUpdate, allowSame, needLock)

    validTransactions := make([]*pb.Transaction, 0)

    if allowSame {
        // Test whether the current working set is still available.
        if m.currentWorking != nil {
            ok := func() bool {
                st := NewBlockChainTStack(m.BC, true, true)
                defer st.Close()

                for _, t := range m.currentWorking.Transactions {
                    if !st.TestAndDo(t) {
                        return false
                    }
                }

                return true
            }()

            if ok {
                validTransactions = m.currentWorking.Transactions
            }
        }
    }

    if len(validTransactions) == 0 {
        // Sleep for a little while for several incoming messages.
        time.Sleep(m.config.Miner.HonestMinerConfig.IncomingWait)

        st := NewBlockChainTStack(m.BC, true, true)
        defer st.Close()

        nrProcessed := 0
        nrMaxProcessed := m.config.Miner.HonestMinerConfig.MaxIncomingProcess

        strategy := m.config.Miner.WorkingSetStrategy
        if strategy == 2 {
            strategy = rand.Intn(2)
        }

        if strategy == 1 {
            pt := m.BC.PendingTransactions
            pt.BeginIter()
            for {
                trans := pt.Next()
                if trans == nil {
                    break
                }

                if st.TestAndDo(trans) {
                    validTransactions = append(validTransactions, trans)
                    pt.MarkSucc(trans)
                } else {
                    pt.MarkFail(trans)
                }
                nrProcessed += 1

                if (len(validTransactions) > 0 && nrProcessed > nrMaxProcessed) || len(validTransactions) == m.config.Common.MaxBlockSize {
                    break
                }
            }
            pt.EndIter()
        } else {
            for _, trans := range m.BC.PendingTransactions.Transactions {
                if st.TestAndDo(trans) {
                    validTransactions = append(validTransactions, trans)
                }
                nrProcessed += 1

                if (len(validTransactions) > 0 && nrProcessed > nrMaxProcessed) || len(validTransactions) == m.config.Common.MaxBlockSize {
                    break
                }
            }
        }
    }

    if len(validTransactions) == 0 {
        // log.Printf("Working set update fails.")
        return false
    }

    // Note:: here, we have BlockMutex.R, UserMutex.R
    block := &pb.Block{
        BlockID: m.BC.LatestBlock.Block.BlockID + 1,
        PrevHash: m.BC.LatestBlock.Hash,
        Transactions: validTransactions,
        MinerID: m.config.Self.ID,
        Nonce: "00000000",
    }
    m.currentWorking = block

    json, err := m.jsonMarshaler.MarshalToString(block)
    if err != nil {
        return false
    }

    presuf := strings.Split(json, "\"Nonce\":\"00000000\"")
    // Sanity check
    if len(presuf) != 2 {
        return false
    }

    // log.Printf("Updating working set: BlockID=%d, Block=%v.", block.BlockID, block)

    prefix, suffix := presuf[0], presuf[1]
    prefix = prefix + "\"Nonce\":\""
    suffix = "\"" + suffix
    for _, w := range m.workers {
        if !w.Working() || forceUpdate {
            w.UpdateWorkingBlock(prefix, suffix)
        }
    }

    return true
}
