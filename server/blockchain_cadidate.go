package main

import (
    "sync"
    pb "../protobuf/go"
)

type BlockChainTStack struct {
    // NOTE:: Owned by transactionsMutex
    Stack []*pb.Transaction
    BC *BlockChain
    Users map[string]*UserInfo

    mutex sync.Mutex
    needLock bool
}

func NewBlockChainTStack(bc *BlockChain, needLock bool) *BlockChainTStack {
    st := &BlockChainTStack{
        Stack: make([]*pb.Transaction, 0),
        BC: bc,
        mutex: &sync.Mutex,
        needLock: needLock
    }

    if needLock {
        st.BC.UserMutex.RLock()
    }
}

func (st *BlockChainTStack) Close() {
    if st.needLock {
        st.BC.UserMutex.RUnlock()
    }
}

func (st *BlockChainTStack) TestAndSet() (succ bool) {
    st.mutex.Lock()
    st.mutex.Unlock()
}

func (st *BlockChainTStack) getMoney(uid string) (money int32) {
    if money, ok := st.Users[]
}

/*
func (bc *BlockChain) PushTransactionStack(t *pb.Transaction, needVerify bool) (err error) {
    // Return nil when succeed.

    // bc.transactionsMutex.Lock()
    // defer bc.transactionsMutex.Unlock()

    if needVerify {
        err = bc.verifyTransaction(t)
        if (err != nil) {
            return
        }
    }

    bc.usersMutex.Lock()
    defer bc.usersMutex.Unlock()

    bc.TransactionStack = append(bc.TransactionStack, t)
    return nil
}
*/
