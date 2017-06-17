package main

import (
    "sync"
)

type MinerWorker interface {
    UpdateWorkingBlock(string prefix, string suffix)
    Mainloop()
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
        master: master,
        prefix: "",
        suffix: "",
        change: make(chan bool),
        mutex: &sync.Mutex{}
    }
}

func (w *SimpleMinerWorker) UpdateWorkingBlock(string prefix, string suffix) {
    w.mutex.Lock()
    defer w.mutex.Unlock()

    w.prefix = prefix
    w.suffix = suffix
    w.change <- true
}

func (w *SimpleMinerWorker) Mainloop() {
    w,working = false
    var prefix string
    var suffix string
    var next int64 = 0

    for {
        var changed: false
        if !w.working {
            changed = <-w.changed
        } else {
            select {
            case _ = <-w.changed:
                changed = true
            default:
                changed = false
            }
        }

        if changed {
            w.working = true

            w.mutex.Lock()
            prefix = prefix
            suffix = suffix
            w.mutex.Unlock()

            next = 0
        }

        for i := 0; i <= 100; i++ {
            nonce := fmt.Sprintf("%08x", next)

            str := prefix + nonce + suffix
            hash := GetHashOfString(str)
            succ := CheckHash(hash)

            if succ {
                m.OnWorkerSuccess(str)
                w.working = false
            }

            next += 1
        }
    }
}

