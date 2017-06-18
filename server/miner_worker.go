package main

import (
    "fmt"
    "log"
    "sync"
    "strings"
    "math/rand"
)

type MinerWorker interface {
    UpdateWorkingBlock(prefix string, suffix string)
    Mainloop()
    Working() bool
}

type SimpleMinerWorker struct {
    master MinerMaster

    // For working set
    prefix string
    suffix string
    change bool
    changec *sync.Cond
    mutex *sync.Mutex
    working bool

    // For work
    rng *rand.Rand
    begin int64
    end int64
}

func NewSimpleMinerWorker(m MinerMaster, begin int64, end int64) (w *SimpleMinerWorker) {
    w = &SimpleMinerWorker{
        master: m,
        prefix: "",
        suffix: "",
        change: false,
        mutex: &sync.Mutex{},
        rng: rand.New(rand.NewSource(rand.Int63())),
        begin: begin,
        end: end,
    }
    w.changec = sync.NewCond(w.mutex)
    log.Printf("Worker %p initialized: Seed=%d, Range=[%d, %d).\n", w, w.rng.Uint32(), w.begin, w.end)
    return
}

func (w *SimpleMinerWorker) UpdateWorkingBlock(prefix string, suffix string) {
    // log.Printf("Updating working block inside worker.")
    w.mutex.Lock()
    defer w.mutex.Unlock()

    // log.Printf("Updating working block inside worker: partial done.")

    w.prefix = prefix
    w.suffix = suffix
    if !w.change {
        w.change = true
        w.changec.Signal()
    }

    // log.Printf("Updating working block inside worker: done.")
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

        w.mutex.Lock()

        if !w.working {
            if !w.change {
                w.changec.Wait()
            }
            changed = true
        } else {
            changed = w.change
        }

        if changed {
            w.working = true
            prefix = w.prefix
            suffix = w.suffix
            next = w.begin
        }
        w.change = false

        if len(prefix) == 0 && len(suffix) == 0 {
            w.working = false
        }

        w.mutex.Unlock()

        if w.working {
            for i := 0; i <= 10000; i++ {
                // nonce := fmt.Sprintf("%08x", w.rng.Uint32())
                nonce := fmt.Sprintf("%08x", next)

                str := strings.Join([]string{prefix, nonce, suffix}, "")
                hash := GetHashString(str)
                succ := CheckHash(hash)

                if succ {
                    w.working = false
                    w.master.OnWorkerSuccess(str, hash)
                    break
                }

                next += 1
            }

            if next >= w.end {
                // NOTE:: WTF???
                w.working = false
            }
        }
    }
}
