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
    OnTransactionAsync(t *pb.Transaction) *OnReceiveResponse
    OnBlockAsync(b *pb.Block) *OnReceiveResponse
}

type HonestMinerMaster struct {
    BC  *BlockChain
    P2P *P2PClient

    config   *ServerConfig
    workers []MinerWorker
}

func NewMinerMaster(c *ServerConfig) (m MinerMaster, e error) {
    switch c.Miner.MinerType {
    case "Honest":
        m = &HonestMinerMaster{
            BC: NewBlockChain(c),
            P2P: NewP2PClient(c),
            config: c,
        }
    default:
        e = fmt.Errorf("Invalid miner type: %s", c.Miner.MinerType)
    }
    return
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

func (m *HonestMinerMaster) OnTransactionAsync(t *pb.Transaction) *OnReceiveResponse {
    response := NewOnReceiveResponse()
    // TODO::
    response.Finish()
    return response
}

func (m *HonestMinerMaster) OnBlockAsync(b *pb.Block) *OnReceiveResponse {
    response := NewOnReceiveResponse()
    // TODO::
    response.Finish()
    return response
}

