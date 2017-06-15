package main

import (
    "fmt"
    "flag"
    "time"
    "math/rand"
)

var id = flag.Int("id", 1, "Server's ID, 1<=ID<=NServers")

// Main function, RPC server initialization
func main() {
    flag.Parse()
    rand.Seed(int64(time.Now().Nanosecond()))
    selfID := fmt.Sprintf("%d",*id)

    config, err := NewServerConfig("config.json", selfID)
    if err != nil {
        panic(err)
    }
    config.Verbose()

    server := NewServer(config)
    server.Mainloop()

    // _ = fmt.Sprintf("Server%02d",*id) //MinerID=
    // _ = hash.GetHashString
}
