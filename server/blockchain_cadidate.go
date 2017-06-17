package main

import (
    pb "../protobuf/go"
)

type BlockChainTStack struct {
    // NOT thread-safe
    Stack []*pb.Transaction
    BC *BlockChain
    UserMoney map[string]int32

    verifyRepeat bool
    needLock bool
}

func NewBlockChainTStack(bc *BlockChain, verifyRepeat bool, needLock bool) *BlockChainTStack {
    st := &BlockChainTStack{
        Stack: make([]*pb.Transaction, 0),
        BC: bc,
        UserMoney: make(map[string]int32),
        verifyRepeat: verifyRepeat,
        needLock: needLock,
    }

    if needLock {
        if verifyRepeat {
            st.BC.BlockMutex.RLock()
        }
        st.BC.UserMutex.RLock()
    }

    return st
}

func (st *BlockChainTStack) Close() {
    if st.needLock {
        st.BC.UserMutex.RUnlock()
        if st.verifyRepeat {
            st.BC.BlockMutex.RUnlock()
        }
    }
}

func (st *BlockChainTStack) Undo(t *pb.Transaction) {
    _ = st.undoTransaction(t)
}

func (st *BlockChainTStack) UndoBlock(bi *BlockInfo) {
    s := bi.Block.Transactions
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
    for _, trans := range bi.Block.Transactions {
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
    return st.BC.GetUserInfoAtomic(uid).Money
}

func (st *BlockChainTStack) verifyTransaction(t *pb.Transaction) (ok bool) {
    balance := st.getMoney(t.FromID)

    if balance < t.Value {
        return false
    }

    if st.verifyRepeat {
        if blocks, ok := st.BC.Trans2Blocks[t.UUID]; ok {
            for _, block := range blocks {
                if block.Valid6 {
                    return false
                }
            }
        }
    }

    return true
}

func (st *BlockChainTStack) doTransaction(t *pb.Transaction) (err error){
    fromMoney := st.getMoney(t.FromID)
    toMoney := st.getMoney(t.ToID)

    st.UserMoney[t.FromID] = fromMoney - t.Value
    st.UserMoney[t.ToID] = toMoney + (t.Value - t.MiningFee)
    return nil
}

func (st *BlockChainTStack) undoTransaction(t *pb.Transaction) (err error) {
    fromMoney := st.getMoney(t.FromID)
    toMoney := st.getMoney(t.ToID)

    st.UserMoney[t.FromID] = fromMoney + t.Value
    st.UserMoney[t.ToID] = toMoney - (t.Value - t.MiningFee)
    return nil
}
