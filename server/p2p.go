package main

import (
    pb "../protobuf/go"

    "google.golang.org/grpc"
)

type RemoteClient struct {
    ServerConfig *RemoteServerConfig
    Conn *grpc.ClientConn
    Client pb.BlockChainMinerClient
    IsAlive bool

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
}

func NewP2PClient(config *ServerConfig) (c *P2PClient) {
    c = &P2PClient{
        // substract 1 (self)
        Clients: make([]*RemoteClient, 0, len(config.Servers) - 1),
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

func (p2pc *P2PClient) RemoteGetBlock() {
    // TODO::
}

func (p2pc *P2PClient) RemoteGetHeight() {
    // TODO::
}

func (p2pc *P2PClient) RemotePushBlock() {
    // TODO::
}

func (p2pc *P2PClient) RemotePushBlockAsync() {
    // TODO::
}

func (p2pc *P2PClient) RemotePushTransaction() {
    // TODO::
}

func (p2pc *P2PClient) RemotePushTransactionAsync() {
    // TODO::
}

