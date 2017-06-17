package main

import (
    "sync"
    pb "../protobuf/go"
)

type BlockChainTStack struct {
    // NOTE:: Owned by transactionsMutex
    Stack []*pb.Transaction
    BC *BlockChain

    mutex sync.Mutex
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
