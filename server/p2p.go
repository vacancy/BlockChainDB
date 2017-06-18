package main

import (
    "fmt"
    "log"
    "time"
    "math/rand"

    pb "../protobuf/go"

    "golang.org/x/net/context"
    "google.golang.org/grpc"
    "github.com/golang/protobuf/proto"
    "github.com/golang/protobuf/jsonpb"
)

type RemoteServerState struct {
    Height int32
}

type RemoteClient struct {
    ServerConfig *RemoteServerConfig
    Conn         *grpc.ClientConn
    Client        pb.BlockChainMinerClient
    IsAlive       bool
    State         RemoteServerState

    RecoverHook   func (height int32, hash string, json string)
    ConnError     error
}

func NewRemoteClient(serverConfig *RemoteServerConfig, pollInterval time.Duration) (rc *RemoteClient) {
    conn, err := grpc.Dial(serverConfig.Addr, grpc.WithInsecure())
    if err != nil {
        rc = &RemoteClient{
            ServerConfig: serverConfig,
            Conn: nil,
            Client: nil,
            IsAlive: false,

            RecoverHook: nil,
            ConnError: err,
        }
    } else {
        rc = &RemoteClient{
            ServerConfig: serverConfig,
            Conn: conn,
            Client: pb.NewBlockChainMinerClient(conn),
            IsAlive: true,
            RecoverHook: nil,
        }
        go rc.Monitor(pollInterval)
    }
    return rc
}

func (rc *RemoteClient) Monitor(pollInterval time.Duration) {
    for {
        if !rc.IsAlive {
            break
        }

        r, err := rc.Client.GetHeight(context.Background(), &pb.Null{})
        if err == nil {
            rc.State.Height = r.Height

            if rc.RecoverHook != nil {
                r2, err2 := rc.Client.GetBlock(context.Background(), &pb.GetBlockRequest{BlockHash: r.LeafHash})
                if err2 == nil {
                    rc.RecoverHook(r.Height, r.LeafHash, r2.Json)
                }
            }
        } else {
            log.Printf("Server polling error: Server=%s, Error=%v.", rc.ServerConfig.ID, err)
        }

        time.Sleep(pollInterval)
    }
}

func (rc *RemoteClient) Close() {
    if rc.IsAlive {
        rc.Conn.Close()
    }
    rc.IsAlive = false
}

type P2PClient struct {
    Clients []*RemoteClient

    config *ServerConfig
}

func NewP2PClient(config *ServerConfig) (c *P2PClient) {
    c = &P2PClient{
        // substract 1 (self)
        Clients: make([]*RemoteClient, 0, len(config.Servers) - 1),
        config: config,
    }

    for _, serverConfig := range config.Servers {
        if serverConfig.ID == config.Self.ID {
            continue
        }
        rc := NewRemoteClient(serverConfig, config.P2P.PollInterval)
        c.Clients = append(c.Clients, rc)
    }

    return c
}

func (p2pc *P2PClient) Close() {
    for _, rc := range p2pc.Clients {
        rc.Close()
    }
}

// Emit Get, Transfer, Verify.

func (p2pc *P2PClient) remoteRequestAsync(funcname string, req proto.Message,
        r *P2PResponse, nrThreads int, timeout time.Duration, needResult bool,
        nrTrials int, retryInterval time.Duration, mask []bool) {

    nrClients := len(p2pc.Clients)
    if nrThreads > nrClients {
        nrThreads = nrClients
    }

    dispatch := func (rc *RemoteClient, req proto.Message) (proto.Message, error) {
        var ctx context.Context
        ctx = context.Background()
        ctx, cancel := context.WithTimeout(ctx, timeout)
        defer cancel()

        switch funcname {
        case "GetBlock":
            return rc.Client.GetBlock(ctx, req.(*pb.GetBlockRequest))
        case "PushBlock":
            return rc.Client.PushBlock(ctx, req.(*pb.JsonBlockString))
        case "PushTransaction":
            return rc.Client.PushTransaction(ctx, req.(*pb.Transaction))
        default:
            return nil, fmt.Errorf("Unknown rpc call %s.", funcname)
        }
    }

    result := make(chan proto.Message, nrThreads)
    done := make(chan bool, nrThreads)
    mapper := func (id int) {
        finished := make(map[int]bool)
        total := int((nrClients - id - 1) / nrThreads) + 1

        for t := 0; t < nrTrials; t++ {
            for j := id; j < nrClients; j += nrThreads {
                if r.AcquiredClose() {
                    break
                }

                if _, ok := finished[j]; ok {
                    continue
                }

                rc := p2pc.Clients[j]
                if !rc.IsAlive || (mask != nil && !mask[j]) {
                    finished[j] = true
                    continue
                }

                res, err := dispatch(rc, req)
                if err == nil {
                    if needResult {
                        result <- res
                    }
                    finished[j] = true
                }
            }

            if len(finished) == total {
                break
            } else {
                time.Sleep(retryInterval)
            }
        }

        // Invoke a nil as a signal of termination
        done <- true
    }

    reducer := func() {
        nrAliveThreads := nrThreads
        for {
            select {
            case msg := <-result:
                if needResult && !r.AcquiredClose() {
                    r.Push(msg)
                }
            case _ = <-done:
                nrAliveThreads -= 1
            }

            if nrAliveThreads == 0 {
                break
            }
        }

        r.Close()

        // Safe to close all channels here
        close(result)
        close(done)
    }

    for i := 0; i < nrThreads; i++ {
        go mapper(i)
    }

    go reducer()
}

func (p2pc *P2PClient) RemoteGetBlock(bid string) *P2PResponse {
    msg := &pb.GetBlockRequest{BlockHash: bid}
    res := NewP2PResponse(p2pc.config.P2P.RequestParallel)

    p2pc.remoteRequestAsync("GetBlock", msg, res,
        p2pc.config.P2P.RequestParallel, p2pc.config.P2P.RequestTimeout,
        true, 1, 0, nil)

    return res
}

func (p2pc *P2PClient) RemotePushBlockAsync(block string) *P2PResponse {
    msg := &pb.JsonBlockString{Json: block}
    // No buffer
    res := NewP2PResponse(1)

    var mask []bool = nil
    if p2pc.config.P2P.PushBlockProbThresh != 0 {
        b := &pb.Block{}
        err := jsonpb.UnmarshalString(block, b)
        if err == nil {
            mask = make([]bool, len(p2pc.Clients))

            lastAlive := -1
            hasSet := false
            for i, rc := range p2pc.Clients {
                if rc.IsAlive {
                    lastAlive = i
                    if (b.BlockID - rc.State.Height) > p2pc.config.P2P.PushBlockProbThresh {
                        mask[i] = (rand.Float32() < p2pc.config.P2P.PushBlockProb)
                    } else {
                        mask[i] = true
                    }

                    if mask[i] {
                        hasSet = true
                    }
                }
            }

            if !hasSet {
                mask[lastAlive] = true
            }
        }

        // maskValues := make([]int, 0)
        // for i, bv := range mask {
        //     if bv {
        //         maskValues = append(maskValues, i)
        //     }
        // }
        // log.Printf("PushBlock mask: %d/%d; %v.", len(maskValues), len(mask), maskValues)
    }

    p2pc.remoteRequestAsync("PushBlock", msg, res,
        p2pc.config.P2P.PushParallel, p2pc.config.P2P.PushTimeout,
        false, p2pc.config.P2P.PushTrials, p2pc.config.P2P.PushRetryInterval, mask)

    return res
}

func (p2pc *P2PClient) RemotePushTransactionAsync(msg *pb.Transaction) *P2PResponse {
    res := NewP2PResponse(p2pc.config.P2P.PushParallel)

    p2pc.remoteRequestAsync("PushTransaction", msg, res,
        p2pc.config.P2P.PushParallel, p2pc.config.P2P.PushTimeout,
        true, p2pc.config.P2P.PushTrials, p2pc.config.P2P.PushRetryInterval, nil)

    return res
}
