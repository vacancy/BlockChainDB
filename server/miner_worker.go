package main

type WorkerInterface interface {
    OnTransaction()
    OnBlock()
    Mainloop()
}

type HonestWorker struct {
}

func NewHonestWorker() (w *HonestWorker) {
    return &HonestWorker{}
}
