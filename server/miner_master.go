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
    GetBlock(bid string) string
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

func (m *BaseMinerMaster) OnClientTransactionAsync(t *pb.Transaction) bool {
    err := m.BC.PushTransaction(t, true)
    return err != nil
}

type HonestMinerMaster struct {
    BaseMinerMaster
    config   *ServerConfig
    workers []MinerWorker
}

func (m *HonestMinerMaster) Recover() (err error) {
    // Recover
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

func (m *HonestMinerMaster) GetBlock(bid string) (json string) {
    // TODO::
    return "{}"
}

func (m *HonestMinerMaster) OnTransactionAsync(t *pb.Transaction) {
    // TODO::
}

func (m *HonestMinerMaster) OnBlockAsync(json string) {
    // TODO::
}

