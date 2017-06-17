package main

import (
    "fmt"
    "log"
    "sync"
)

type MinerWorker interface {
    UpdateWorkingBlock(prefix string, suffix string)
    Mainloop()
    Working() bool
}

type SimpleMinerWorker struct {
    master MinerMaster
    prefix string
    suffix string
    change chan bool
    mutex *sync.Mutex
    working bool
}

func NewSimpleMinerWorker(m MinerMaster) (w *SimpleMinerWorker) {
    return &SimpleMinerWorker{
        master: m,
        prefix: "",
        suffix: "",
        change: make(chan bool),
        mutex: &sync.Mutex{},
    }
}

func (w *SimpleMinerWorker) UpdateWorkingBlock(prefix string, suffix string) {
    w.mutex.Lock()
    defer w.mutex.Unlock()

    w.prefix = prefix
    w.suffix = suffix
    w.change <- true
}

func (w *SimpleMinerWorker) Working() bool {
    return w.working
}

func (w *SimpleMinerWorker) Mainloop() {
    w.working = false
    var prefix string
    var suffix string
    var next int64 = 0

    for {
        changed := false
        if !w.working {
            changed = <-w.change
        } else {
            select {
            case _ = <-w.change:
                changed = true
            default:
                changed = false
            }
        }

        if changed {
            w.working = true

            w.mutex.Lock()

            // Remove all messages from the chan
            (func() {
                for {
                    select {
                    case _ = <-w.change:
                        // pass
                    default:
                        return
                    }
                }
            })()

            prefix = w.prefix
            suffix = w.suffix
            w.mutex.Unlock()

            next = 0
        }

        if w.working {
            for i := 0; i <= 100; i++ {
                nonce := fmt.Sprintf("%08x", next)

                str := prefix + nonce + suffix
                hash := GetHashString(str)
                succ := CheckHash(hash)

                if succ {
                    w.working = false
                    w.master.OnWorkerSuccess(str)
                    break
                }

                next += 1
            }
        }
    }
}
