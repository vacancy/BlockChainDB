package main

import (
    "sync"
    "fmt"
    "strings"

    pb "../protobuf/go"
    "github.com/golang/protobuf/jsonpb"
)

type UserInfo struct {
    Money int32
}

type BlockInfo struct {
    Hash string
    Json string
    Block *pb.Block

    //Valid6: on the longest branch
    Valid6 bool
}

type BlockChain struct {
    // hash(jsonify(b)) -> bi
    Blocks       map[string]*BlockInfo
    LatestBlock *BlockInfo

    // t.UUID -> []*BlockInfo
    // NOTE:: Owned by BlockMutex
    Trans2Blocks map[string][]*BlockInfo

    // NOTE:: Owned by TransactionMutex
    PendingTransactions map[string]*pb.Transaction

    // UserID -> money
    Users map[string]*UserInfo

    // Callbacks
    // OnChangeLatestCallbacks []func ()
    // OnAddActiveCallbacks []func (string)
    // OnRemoveActiveCallbacks []func (string)

    config            *ServerConfig
    p2pc              *P2PClient

    // Mutexes are acuiqred in this order:
    //   Block -> User -> Transaction
    BlockMutex        *sync.RWMutex
    UserMutex         *sync.RWMutex
    TransactionMutex  *sync.RWMutex

    defaultUserInfo   *UserInfo
}

func NewBlockChain(c *ServerConfig, p2pc *P2PClient) (bc *BlockChain) {
    bc = &BlockChain{
        Blocks: make(map[string]*BlockInfo),
        LatestBlock: &BlockInfo{
            Json: "{}",
            Block: &pb.Block{BlockID: 0},
            Hash: strings.Repeat("0", 64),
            Valid6: true,
        },
        Trans2Blocks: make(map[string][]*BlockInfo),

        Users: make(map[string]*UserInfo),

        PendingTransactions: make(map[string]*pb.Transaction),

        config: c,
        p2pc: p2pc,

        BlockMutex: &sync.RWMutex{},
        UserMutex: &sync.RWMutex{},
        TransactionMutex: &sync.RWMutex{},

        defaultUserInfo: &UserInfo{Money: c.Common.DefaultMoney},
    }

    bc.Blocks[strings.Repeat("0", 64)] = bc.LatestBlock
    return
}

// Public methods: block

func (bc *BlockChain) GetBlock(hash string) (*BlockInfo, bool) {
    bc.BlockMutex.RLock()
    defer bc.BlockMutex.RUnlock()
    bi, ok := bc.Blocks[hash]
    return bi, ok
}

func (bc *BlockChain) GetLatestBlock() (bi *BlockInfo) {
    bc.BlockMutex.RLock()
    defer bc.BlockMutex.RUnlock()
    return bc.LatestBlock
}

func (bc *BlockChain) PushBlockJson(json string) (lastChanged bool, err error) {
    return bc.pushBlockJsonInternal(json, true)
}

func (bc *BlockChain) DeclareBlockJson(json string) (lastChanged bool, err error) {
    return bc.pushBlockJsonInternal(json, false)
}

func (bc *BlockChain) pushBlockJsonInternal(json string, needVerifyInfo bool) (lastChanged bool, err error) {
    block := &pb.Block{}
    err = jsonpb.UnmarshalString(json, block)

    if err != nil {
        return false, err
    }

    bi := &BlockInfo{
        Json: json,
        Hash: GetHashString(json),
        Block: block,
        Valid6: false,
    }

    lastChanged = false

    // Return nil when succeed.
    if (needVerifyInfo) {
        err = bc.verifyBlockInfo(bi)
        if (err != nil) {
            return
        }
    }

    bc.BlockMutex.Lock()
    defer bc.BlockMutex.Unlock()
    bc.UserMutex.Lock()
    defer bc.UserMutex.Unlock()
    bc.TransactionMutex.Lock()
    defer bc.TransactionMutex.Unlock()

    return bc.refreshBlockChain(bi)
}

func (bc *BlockChain) PushTransaction(t *pb.Transaction, needVerifyInfo bool) (rc int) {
    // Verify transaction based on current info
    // RC: 0=fail, 1=ok, 2=already-in-current-chain

    if (needVerifyInfo) {
        if err := bc.verifyTransactionInfo(t); err != nil {
            return 0
        }
    }

    bc.BlockMutex.RLock()
    defer bc.BlockMutex.RUnlock()

    bc.TransactionMutex.RLock()
    defer bc.TransactionMutex.RUnlock()

    if err := bc.verifyTransactionUUID(t); err != nil {
        return 0
    }

    if _, ok := bc.PendingTransactions[t.UUID]; ok {
        return 0
    }

    if err := bc.verifyTransactionRepeat(t); err != nil {
        return 2
    }

    // TODO:: whether we need to verify or not
    // bc.UserMutex.RLock()
    // defer bc.UserMutex.RUnlock()

    // err := bc.verifyTransactionMoney(t)
    // if (err != nil) {
    //     return 0
    // }

    bc.PendingTransactions[t.UUID] = t
    return 1
}

func (bc *BlockChain) VerifyTransaction6(t *pb.Transaction) (rc int, hash string) {
    // Return return code and hash
    // Return code: 0=fail; 1=peding; 2=succeed.

    bc.BlockMutex.RLock()
    defer bc.BlockMutex.RUnlock()

    bc.TransactionMutex.RLock()
    defer bc.BlockMutex.RUnlock()

    if err := bc.verifyTransactionUUID(t); err != nil {
        return 0, "?"
    }

    if blocks, ok := bc.Trans2Blocks[t.UUID]; ok {
        for _, block := range blocks {
            if block.Valid6 && block.Block.BlockID >= bc.LatestBlock.Block.BlockID - 6 {
                return 2, block.Hash
            }
        }
        return 1, blocks[0].Hash
    }

    // TODO:: what about the transactions in PendingTransactions
    // TODO(IMPORTANT)::

    // TODO:: implement checking of side-chain
    // TODO(FUCK)::

    return 0, "?"
}

func (bc *BlockChain) SetDefaultUserInfo(uid string) (u *UserInfo) {
    bc.UserMutex.Lock()
    defer bc.UserMutex.Unlock()

    return bc.setDefaultUserInfo(uid)
}

func (bc *BlockChain) GetUserInfo(uid string) (u *UserInfo) {
    bc.UserMutex.RLock()
    defer bc.UserMutex.RUnlock()

    return bc.GetUserInfoAtomic(uid)
}

func (bc *BlockChain) GetUserInfoAtomic(uid string) (u *UserInfo) {
    u, ok := bc.getUserInfo(uid)
    if !ok {
        return bc.defaultUserInfo
    }
    return u
}

// Private: Execution

func (bc *BlockChain) doBlock(x *BlockInfo) {
    x.Valid6 = true
    for _, trans := range x.Block.Transactions {
        bc.doTransaction(trans)
    }
}

func (bc *BlockChain) undoBlock(x *BlockInfo) {
    x.Valid6 = false
    s := x.Block.Transactions
    for i := len(s) - 1; i >= 0; i-- {
        bc.undoTransaction(s[i])
    }
}

func (bc *BlockChain) doTransaction(t *pb.Transaction) (err error) {
    // Require: UserMutex, TransactionMutex.
    bc.setDefaultUserInfo(t.FromID).Money -= t.Value
    bc.setDefaultUserInfo(t.ToID).Money += t.Value - t.MiningFee
    delete(bc.PendingTransactions, t.UUID)
    return nil
}

func (bc *BlockChain) undoTransaction(t *pb.Transaction) (err error) {
    // Require: UserMutex, TransactionMutex.
    bc.setDefaultUserInfo(t.FromID).Money += t.Value
    bc.setDefaultUserInfo(t.ToID).Money -= t.Value - t.MiningFee
    bc.PendingTransactions[t.UUID] = t
    return nil
}

// Private: Block

func (bc *BlockChain) addBlock(bi *BlockInfo) {
    // Add a verified (info only, no transactions) into the database
    // Require BlockMutex

    bc.Blocks[bi.Hash] = bi
    for _, trans := range bi.Block.Transactions {
        blocks := bc.Trans2Blocks[trans.UUID]
        if blocks == nil {
            blocks = make([]*BlockInfo, 0)
        }
        blocks = append(blocks, bi)
        bc.Trans2Blocks[trans.UUID] = blocks
    }
}

func (bc *BlockChain) refreshBlockChain(bi *BlockInfo) (latestChanged bool, err error) {
    // Handle chain switch and go through the transactions in `b`
    bc.addBlock(bi)

    b := bi.Block
    height := bc.LatestBlock.Block.BlockID
    latestChanged = b.BlockID > height || b.BlockID == height && bi.Hash < bc.LatestBlock.Hash
    if !latestChanged {
        return
    }

    if b.BlockID == height + 1 && b.PrevHash == bc.LatestBlock.Hash {
        latestChanged = bc.extendLatestBlock(bi)
    } else {
        latestChanged = bc.switchLatestBlock(bc.LatestBlock, bi)
    }
    return
}

func (bc *BlockChain) extendLatestBlock(bi *BlockInfo) (succ bool) {
    if err := bc.verifyBlockTransaction(bi); err == nil {
        bc.doBlock(bi)
        bc.LatestBlock = bi
        return true
    }
    return false
}

func (bc *BlockChain) switchLatestBlock(x *BlockInfo, y *BlockInfo) (succ bool) {
    z := bc.findBlockLCA(x, y)

    succ = bc.switchLatestBlock_complete(y)
    if !succ {
        return
    }

    // TODO:: delete inValid6 block
    // NOTE(MJY):: Need undo "Trans2Blocks" too.

    undos := make([]*BlockInfo, 0)
    for x != z {
        undos = append(undos, x)
        x = bc.Blocks[x.Block.PrevHash]
    }
    dos := make([]*BlockInfo, 0)
    for y != z {
        dos = append(dos, y)
        y = bc.Blocks[y.Block.PrevHash]
    }

    succ = bc.switchLatestBlock_undodo(undos, dos)
    if !succ {
        return
    }

    bc.LatestBlock = y
    return true
}

func (bc *BlockChain) findBlockLCA(x *BlockInfo, y *BlockInfo) (z *BlockInfo) {
    for x != y {
        if x.Block.BlockID > y.Block.BlockID {
            x = bc.Blocks[x.Block.PrevHash]
        } else {
            y = bc.Blocks[y.Block.PrevHash]
        }
    }
    return x
}

func (bc *BlockChain) switchLatestBlock_complete(bi *BlockInfo) (succ bool) {
    // complete the sub-blockchain
    // Require BlockMutex

    for {
        prev := bi.Block.PrevHash
        if _, ok := bc.Blocks[prev]; ok {
            break
        }

        response := bc.p2pc.RemoteGetBlock(prev)
        for {
            msg := response.Get()
            if msg == nil {
                return false
            }

            json := (msg.(*pb.JsonBlockString)).Json
            block := &pb.Block{}
            err := jsonpb.UnmarshalString(json, block)

            if err != nil {
                continue
            }

            newBi := &BlockInfo{
                Json: json,
                Hash: GetHashString(json),
                Block: block,
                Valid6: false,
            }

            if prev != newBi.Hash {
                continue
            }

            if err = bc.verifyBlockInfo(newBi); err != nil {
                bc.Blocks[prev] = newBi
                response.AcquireClose()
                break
            }
        }

        bi = bc.Blocks[prev]
    }

    return true
}

func (bc *BlockChain) switchLatestBlock_undodo(undos []*BlockInfo, dos []*BlockInfo) (succ bool) {
    // Undo and do; once fail, rollback.
    for _, bi := range undos {
        bc.undoBlock(bi)
    }

    succ = true
    for i := len(dos) - 1; i >= 0; i-- {
        bi := dos[i]

        if err := bc.verifyBlockTransaction(bi); err == nil {
            bc.doBlock(bi)
        } else {
            for j := i + 1; j < len(dos); j++ {
                bc.undoBlock(dos[j])
            }
            succ = false
            break
        }
    }

    if !succ {
        for i := len(undos) - 1; i >= 0; i-- {
            bc.doBlock(undos[i])
        }
    }

    return succ
}

// Private: Block verifications

func (bc *BlockChain) verifyBlockInfo(bi *BlockInfo) (err error) {
    if !CheckHash(bi.Hash) {
        return fmt.Errorf("Verify block failed, inValid6 hash: %s.", bi.Hash)
    }

    // Check hex
    if !CheckNonce(bi.Block.Nonce) {
        return fmt.Errorf("Verify block failed, inValid6 nonce: %s.", bi.Block.Nonce)
    }

    // Check basic transaction info
    for _, t := range bi.Block.Transactions {
        if err = bc.verifyTransactionInfo(t); err != nil {
            return err
        }
    }

    return nil
}

func (bc *BlockChain) verifyBlockTransaction(bi *BlockInfo) (err error) {
    // We don't need to only verify basic info here
    // Return nil when succeed.
    // Require: BlockMutex.R, TransactionMutex.R, UserMutex.R
    b := bi.Block

    // Check used transaction
    for _, t := range b.Transactions {
        if err = bc.verifyTransactionUUID(t); err != nil {
            return err
        }
        if err = bc.verifyTransactionRepeat(t); err != nil {
            return err
        }
    }

    // Check Valid6ity
    stack := NewBlockChainTStack(bc, false, false)
    for _, t := range b.Transactions {
        if !stack.TestAndDo(t) {
            return fmt.Errorf("Verify block failed, transaction amount inValid6: %s.", t.UUID)
        }
    }
    stack.Close()

    return nil
}

// Private: Transaction

func (bc *BlockChain) verifyTransactionInfo(t *pb.Transaction) (err error) {
    // Return nil when succeed.

    // TODO:: check len(userid) == 8
    // TODO:: check from != to

    if t.Type != pb.Transaction_TRANSFER {
        return fmt.Errorf("Verify transaction failed, unsupported type: %s.", t.Type)
    }
    if t.MiningFee <= 0 {
        return fmt.Errorf("Verify transaction failed, non-positive mining fee: %d", t.MiningFee)
    }
    if t.Value < 0 {
        return fmt.Errorf("Verify transaction failed, negative value: %d", t.Value)
    }
    if t.Value <= t.MiningFee {
        return fmt.Errorf("Verify transaction failed, insufficient value: %d, mining fee: %d.", t.Value, t.MiningFee)
    }

    return nil
}

func (bc *BlockChain) verifyTransactionUUID(t *pb.Transaction) (err error) {
    // Verify whether there is some transaction with same UUID but different value.
    // Return nil when succeed.
    // Require: BlockMutex.R, TransactionMutex.R
    // TODO:: FUCKKKKKK

    return nil
}

func (bc *BlockChain) verifyTransactionRepeat(t *pb.Transaction) (err error) {
    // Verify whether the transaction has already appeared on the current chain.
    // Return nil when succeed.
    // Require: BlockMutex.R

    if blocks, ok := bc.Trans2Blocks[t.UUID]; ok {
        for _, block := range blocks {
            if block.Valid6 {
                return fmt.Errorf("Verify block failed, transaction exists: %s.", t.UUID)
            }
        }
    }
    return nil
}

func (bc *BlockChain) verifyTransactionMoney(t *pb.Transaction) (err error) {
    // Verify whether the transaction can be done (considering user accounts).
    // Require: UserMutex.R
    balance := bc.GetUserInfoAtomic(t.FromID).Money
    if balance < t.Value {
        return fmt.Errorf("Verify transaction money failed, insufficient balance: %d, transfer money: %d.",
                balance, t.Value)
    }
    return nil
}

// Private: User

func (bc *BlockChain) getUserInfo(uid string) (u *UserInfo, ok bool) {
    u, ok = bc.Users[uid]
    return
}

func (bc *BlockChain) setDefaultUserInfo(uid string) (u *UserInfo) {
    u, ok := bc.Users[uid]
    if !ok {
        u = &UserInfo{Money: bc.config.Common.DefaultMoney}
        bc.Users[uid] = u
    }
    return
}
