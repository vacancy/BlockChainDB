package main

import (
    "fmt"
    "time"

    pb "../protobuf/go"

    "golang.org/x/net/context"
    "google.golang.org/grpc"
    "github.com/golang/protobuf/proto"
)

type RemoteClient struct {
    ServerConfig *RemoteServerConfig
    Conn         *grpc.ClientConn
    Client        pb.BlockChainMinerClient
    IsAlive       bool

    ConnError error
}

func NewRemoteClient(serverConfig *RemoteServerConfig) (rc *RemoteClient) {
    conn, err := grpc.Dial(serverConfig.Addr, grpc.WithInsecure())
    if err != nil {
        rc = &RemoteClient{
            ServerConfig: serverConfig,
            Conn: nil,
            Client: nil,
            IsAlive: false,

            ConnError: err,
        }
    } else {
        rc = &RemoteClient{
            ServerConfig: serverConfig,
            Conn: conn,
            Client: pb.NewBlockChainMinerClient(conn),
            IsAlive: true,
        }
    }
    return rc
}

func (rc *RemoteClient) Close() {
    if rc.IsAlive {
        rc.Conn.Close()
    }
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
        rc := NewRemoteClient(serverConfig)
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
        nrTrials int, retryInterval time.Duration) {

    nrClients := len(p2pc.Clients)

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
        for t := 0; t < nrTrials; t++ {
            for j := id; j < nrClients; j += nrThreads {
                if r.AcquiredClose() {
                    break
                }

                if _, ok := finished[j]; ok {
                    continue
                }

                // TODO:: choose server sequence randomly
                rc := p2pc.Clients[j]
                if !rc.IsAlive {
                    finished[j] := true
                    continue
                }

                res, err := dispatch(rc, req)
                if err == nil {
                    if needResult {
                        result <- res
                    }
                    finished[j] := true
                }
            }

            if len(unfinished) == 0 {
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
    res := NewP2PResponse()

    p2pc.remoteRequestAsync("GetBlock", msg, res,
        p2pc.config.P2P.RequestParallel, p2pc.config.P2P.RequestTimeout,
        true, 1, 0)

    return res
}

func (p2pc *P2PClient) RemotePushBlockAsync(block string) *P2PResponse {
    msg := &pb.JsonBlockString{Json: block}
    res := NewP2PResponse()

    p2pc.remoteRequestAsync("PushBlock", msg, res,
        p2pc.config.P2P.PushParallel, p2pc.config.P2P.PushTimeout,
        false, p2pc.config.P2P.PushTrials, p2pc.config.P2P.PushRetryInterval)

    return res
}

func (p2pc *P2PClient) RemotePushTransactionAsync(msg *pb.Transaction) *P2PResponse {
    res := NewP2PResponse()

    p2pc.remoteRequestAsync("PushTransaction", msg, res,
        p2pc.config.P2P.PushParallel, p2pc.config.P2P.PushTimeout,
        true, p2pc.config.P2P.PushTrials, p2pc.config.P2P.PushRetryInterval)

    return res
}

