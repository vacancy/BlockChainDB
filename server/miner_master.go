package main

import (
    "fmt"
    "log"
    "time"

    pb "../protobuf/go"
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
    Mainloop()

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
}

func NewMinerMaster(c *ServerConfig) (m MinerMaster, e error) {
    p2pc := NewP2PClient(c)

    switch c.Miner.MinerType {
    case "Honest":
        m = &HonestMinerMaster{
            BaseMinerMaster: BaseMinerMaster{NewBlockChain(c, p2pc), p2pc},
            config: c,
        }
    default:
        e = fmt.Errorf("Invalid miner type: %s", c.Miner.MinerType)
    }
    return
}

type BaseMinerMaster struct {
    BC  *BlockChain
    P2P *P2PClient
}

func (m *BaseMinerMaster) GetUserInfo(uid string) *UserInfo {
    return m.BC.GetUserInfoWithDefault(uid)
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
}

func (m *HonestMinerMaster) Recover() (err error) {
    // TODO:: 
    return nil
}

func (m *HonestMinerMaster) Mainloop() {
    // Start workers
    for i := 0; i < m.config.Miner.NrWorkers; i++ {
        log.Printf("Starting worker #%d", i)
        w := NewSimpleMinerWorker(m)
        m.workers = append(m.workers, w)
        go w.Mainloop()
    }

    for {
        // Mainloop here
        time.Sleep(1000)
    }
}

func (m *HonestMinerMaster) GetBlock(bid string) *BlockInfo {
    b, ok := m.BC.GetBlock(bid)
    if !ok {
        return nil
    }
    return b
}

func (m *HonestMinerMaster) OnClientTransactionAsync(t *pb.Transaction) bool {
    return m.processTransaction(t)
}

func (m *HonestMinerMaster) OnTransactionAsync(t *pb.Transaction) {
    _ = m.processTransaction(t)
}

func (m *HonestMinerMaster) OnBlockAsync(json string) {
    oldLatest := m.BC.GetLatestBlock()
    // TODO:: 
    _, err := m.BC.PushBlockJson(json)
    if err == nil {
        newLatest := m.BC.GetLatestBlock()
        if oldLatest.Hash != newLatest.Hash {
            // TODO:: Notify
        }
    }
}

func (m *HonestMinerMaster) processTransaction(t *pb.Transaction) bool {
    // TODO:: Flow control

    // err := m.BC.PushTransaction(t, true)
    var err error = nil
    if err != nil {
        return false
    }

    // TODO:: Notify
    return true
}

func (m *HonestMinerMaster) gatherBatch() bool {
    return true
}
