package main

import (
    "container/heap"

    pb "../protobuf/go"
)

type TPQ []*pb.Transaction

func (pq TPQ) Len() int {
    return len(pq)
}
func (pq TPQ) Less(i, j int) bool {
    return pq[i].MiningFee > pq[j].MiningFee
}
func (pq TPQ) Swap(i, j int) {
    pq[i], pq[j] = pq[j], pq[i]
}
func (pq *TPQ) Push(x interface{}) {
    item := x.(*pb.Transaction)
    *pq = append(*pq, item)
}
func (pq *TPQ) Pop() interface{} {
    old := *pq
    n := len(old)
    item := old[n-1]
    *pq = old[0 : n-1]
    return item
}

type PriorityTransactionPool struct {
    Transactions map[string]*pb.Transaction
    MajorQueue *TPQ
    Succs []*pb.Transaction
    Fails []*pb.Transaction

    iterating int
    failIndex int
}

func NewPriorityTransactionPool() *PriorityTransactionPool {
    return &PriorityTransactionPool{
        Transactions: make(map[string]*pb.Transaction),
        MajorQueue: &TPQ{},
        Succs: make([]*pb.Transaction, 0),
        Fails: make([]*pb.Transaction, 0),
    }
}

func (p *PriorityTransactionPool) Add(t *pb.Transaction) {
    if _, ok := p.Transactions[t.UUID]; ok {
        return
    }
    p.Transactions[t.UUID] = t
    heap.Push(p.MajorQueue, t)
}

func (p *PriorityTransactionPool) Remove(t *pb.Transaction) {
    // Lazy remove in heaps
    delete(p.Transactions, t.UUID)
}

func (p *PriorityTransactionPool) Has(t *pb.Transaction) bool {
    _, ok := p.Transactions[t.UUID]
    return ok
}

func (p *PriorityTransactionPool) BeginIter() {
    // Check it
    p.iterating = 0
    p.failIndex = 0
}

func (p *PriorityTransactionPool) Next() (t *pb.Transaction) {
    if len(p.Transactions) == 0 {
        return nil
    }
    if p.iterating >= len(p.Transactions) {
        return nil
    }

    for {
        t := p.maybeNext()
        if t == nil {
            continue
        }

        p.iterating += 1
        return t
    }
}

func (p *PriorityTransactionPool) MarkSucc(t *pb.Transaction) {
    // log.Printf("  Put succ trans: %s Value=%d.", t.FromID, t.Value)
    p.Succs = append(p.Succs, t)
}

func (p *PriorityTransactionPool) MarkFail(t *pb.Transaction) {
    // log.Printf("  Put fail tran: %s Value=%d.", t.FromID, t.Value)
    p.Fails = append(p.Fails, t)
}

func (p *PriorityTransactionPool) maybeNext() (item *pb.Transaction) {
    if p.MajorQueue.Len() == 0 {
        item = p.Fails[p.failIndex]
        p.failIndex += 1
    } else {
        item = heap.Pop(p.MajorQueue).(*pb.Transaction)
    }
    if p.Has(item) {
        return item
    }
    return nil
}

func (p *PriorityTransactionPool) EndIter() {
    for _, t := range p.Succs {
        heap.Push(p.MajorQueue, t)
    }
    p.Succs = make([]*pb.Transaction, 0)
    p.Fails = p.Fails[p.failIndex:]
    // if p.failIndex == len(p.Fails) {
    //  for _, t := range p.Fails {
    //      heap.Push(p.MajorQueue, t)
    //  }
    //  p.Fails = make([]*pb.Transaction, 0)
    // }
}
