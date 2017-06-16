package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
)

type CommonConfig struct {
    MaxBlockSize int
    DefaultMoney int32
}

type RemoteServerConfig struct {
    ID string
    Addr string
    DataDir string
}

type SnapshotConfig struct {
    SnapshotInterval int
}

type MinerConfig struct {
    MinerType string
    NrWorkers int
}

type ServerConfig struct {
    Common    *CommonConfig
    Servers []*RemoteServerConfig
    Self      *RemoteServerConfig
    Snapshot  *SnapshotConfig
    Miner     *MinerConfig
}

func NewServerConfig(configFilename string, selfID string) (config *ServerConfig, err error) {
    configJson, err := ioutil.ReadFile(configFilename)
    if err != nil {
        return
    }

    var allServers map[string]interface{}
    err = json.Unmarshal(configJson, &allServers)
    if err != nil {
        return
    }
    nrServers := int(allServers["nservers"].(float64))

    config = &ServerConfig{}

    config.Servers = make([]*RemoteServerConfig, 0, nrServers)

    for i := 1; i <= nrServers; i++ {
        serverID := fmt.Sprintf("%d", i)
        serverConfig := allServers[serverID].(map[string]interface{})
        thisConfig := &RemoteServerConfig{
            ID: serverID,
            Addr: fmt.Sprintf("%s:%s", serverConfig["ip"], serverConfig["port"]),
            DataDir: fmt.Sprintf("%s", serverConfig["dataDir"]),
        }

        config.Servers = append(config.Servers, thisConfig)
        if serverID == selfID {
            config.Self = thisConfig
        }
    }

    if config.Self == nil {
        err = fmt.Errorf("Unknwon server ID: %s.", selfID)
        return
    }

    config.Common = &CommonConfig{
        MaxBlockSize: 50,
        DefaultMoney: 1000,
    }

    config.Snapshot = &SnapshotConfig {
        SnapshotInterval: 0,
    }

    config.Miner = &MinerConfig {
        MinerType: "Honest",
        NrWorkers: 1,
    }

    return
}

func (config *ServerConfig) Verbose() {
    log.Println("Server configuration")
    log.Println("--------------------------------")

    log.Println("Common configuration")
    log.Printf("- MaxBlockSize: %d\n", config.Common.MaxBlockSize)
    log.Printf("- DefaultMoney: %d\n", config.Common.DefaultMoney)
    log.Println("")

    log.Println("Snapshot configuration")
    log.Printf("- SnapshotInterval : %d\n", config.Snapshot.SnapshotInterval)
    log.Println("")

    log.Println("Self configuration")
    log.Printf("- Server ID: %s\n", config.Self.ID)
    log.Printf("- Server Addr: %s\n", config.Self.Addr)
    log.Printf("- Server DataDir: %s\n", config.Self.DataDir)
    log.Println("")

    log.Println("Miner configuration")
    log.Printf("- MinerType: %s\n", config.Miner.MinerType)
    log.Printf("- NrWorkers: %s\n", config.Miner.NrWorkers)
    log.Println("")

    log.Println("Servers configuration")
    for i, remoteConfig := range config.Servers {
        log.Printf("- Server index #%d\n", i)
        log.Printf("  - Server ID: %s\n", remoteConfig.ID)
        log.Printf("  - Server Addr: %s\n", remoteConfig.Addr)
        log.Printf("  - Server DataDir: %s\n", remoteConfig.DataDir)
    }
    log.Println("")
}
