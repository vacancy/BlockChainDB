package main

import (
    pb "../protobuf/go"
)

type BlockChainTStack struct {
    // NOT thread-safe
    Stack []*pb.Transaction
    BC *BlockChain
    UserMoney map[string]int32

    needLock bool
}

func NewBlockChainTStack(bc *BlockChain, needLock bool) *BlockChainTStack {
    st := &BlockChainTStack{
        Stack: make([]*pb.Transaction, 0),
        BC: bc,
        needLock: needLock,
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

func (st *BlockChainTStack) Undo(t *pb.Transaction) {
    _ := st.undoTransaction(t)
}

func (st *BlockChainTStack) UndoBlock(bi *BlockInfo) {
    s := x.Block.Transactions
    for i := len(s) - 1; i >= 0; i-- {
        st.Undo(s[i])
    }
}

func (st *BlockChainTStack) TestAndDo(t *pb.Transaction) (succ bool) {
    if ok := st.verifyTransaction(t); ok {
        st.doTransaction(t)
        return true
    }
    return false
}

func (st *BlockChainTStack) TestAndDoBlock(bi *BlockInfo) (succ bool) {
    for _, trans := range x.Block.Transactions {
        if ok := st.TestAndDo(trans); !ok {
            return false
        }
    }
    return true
}

func (st *BlockChainTStack) getMoney(uid string) (money int32) {
    if money, ok := st.UserMoney[uid]; ok {
        return money
    }
    return st.BC.GetUserInfoWithDefault(uid).Money
}

func (st *BlockChainTStack) verifyTransaction(t *pb.Transaction) (ok bool) {
    balance := st.getMoney(t.FromID)
    return balance >= t.Value
}

func (st *BlockChainTStack) doTransaction(t *pb.Transaction) (err error){
    fromMoney := st.getMoney(t.FromID)
    toMoney := st.getMoney(t.ToID)

    UserMoney[t.FromID] = fromMoney - t.Value
    UserMoney[t.ToID] = toMoney + (t.Value - t.MiningFee)
    return nil
}

func (st *BlockChainTStack) undoTransaction(t *pb.Transaction) (err error) {
    fromMoney := st.getMoney(t.FromID)
    toMoney := st.getMoney(t.ToID)

    UserMoney[t.FromID] = fromMoney + t.Value
    UserMoney[t.ToID] = toMoney - (t.Value - t.MiningFee)
    return nil
}
