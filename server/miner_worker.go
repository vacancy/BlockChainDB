package main

type MinerWorker interface {
    Mainloop()
}

type SimpleMinerWorker struct {
}

func NewSimpleMinerWorker(m MinerMaster) (w *SimpleMinerWorker) {
    return &SimpleMinerWorker{}
}

func (w *SimpleMinerWorker) Mainloop() {

}

