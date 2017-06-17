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
    //Valid: on the longest branch
    Valid bool
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

    jsonMarshaler     *jsonpb.Marshaler
    defaultUserInfo   *UserInfo
}

func NewBlockChain(c *ServerConfig, p2pc *P2PClient) (bc *BlockChain) {
    return &BlockChain{
        Blocks: make(map[string]*BlockInfo),
        LatestBlock: &BlockInfo{
            Json: "{}",
            Block: &pb.Block{BlockID: 0},
            Hash: strings.Repeat("0", 64),
            Valid: true,
        },
        Trans2Blocks: make(map[string][]*BlockInfo),

        Users: make(map[string]*UserInfo),

        PendingTransactions: make(map[string]*pb.Transaction),

        config: c,
        p2pc: p2pc,

        BlockMutex: &sync.RWMutex{},
        UserMutex: &sync.RWMutex{},
        TransactionMutex: &sync.RWMutex{},

        jsonMarshaler: &jsonpb.Marshaler{EnumsAsInts: false},
        defaultUserInfo: &UserInfo{Money: bc.config.Common.DefaultMoney},

        // OnChangeLatestCallbacks: make([]func (), 0)
                    // OnAddActiveCallbacks: make([]func (string), 0)
        // OnRemoveActiveCallbacks: make([]func (string), 0)
    }
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
    block := &pb.Block{}
    jsonpb.UnmarshalString(json, block)

    bi := &BlockInfo{
        Json: json,
        Hash: GetHashString(json),
        Block: block,
        Valid: false,
    }

    return bc.pushBlockInfo(bi, true)
}
            

func (bc *BlockChain) DeclareBlockInfo(bi *BlockInfo) (lastChanged bool, err error) {
    return bc.pushBlockInfo(bi, false)
}

func (bc *BlockChain) pushBlockInfo(bi *BlockInfo, needVerify bool) (lastChanged bool, err error) {
    lastChanged = false

    bc.BlockMutex.Lock()
    defer bc.BlockMutex.Unlock()
    bc.UserMutex.Lock()
    defer bc.UserMutex.Unlock()

    // Return nil when succeed.
    if (needVerify) {
        err = bc.verifyBlock(bi)
        if (err != nil) {
            return
        }
    }

    bc.TransactionMutex.Lock()
    defer bc.TransactionMutex.Unlock()

    return bc.refreshBlockChain(bi)
}

func (bc *BlockChain) PushTransaction(t *pb.Transaction, needVerify bool) (rc int) {
    // Verify transaction based on current info
    // RC: 0=fail, 1=ok, 2=already-in-block

    if (needVerify) {
        err := bc.verifyTransactionInfo(t)
        if (err != nil) {
            return 0
        }
    }

    // always acquire block mutex first
    // TODO:: check if the t == getTByUUID(t.UUID)
    bc.BlockMutex.RLock()
    defer bc.BlockMutex.RUnlock()

    if _, ok := bc.Trans2Blocks[t.UUID]; ok {
        return 2
    }

    bc.UserMutex.RLock()
    defer bc.UserMutex.RUnlock()

    err := bc.verifyTransactionMoney(t)
    if (err != nil) {
        return 0
    }

    // TODO:: done
    bc.PendingTransactions[t.UUID] = t
    return 1
}

func (bc *BlockChain) VerifyTransaction6(t *pb.Transaction) (rc int, hash string) {
    // Return return code and hash
    // Return code: 0=fail; 1=peding; 2=succeed.

    bc.BlockMutex.RLock()
    defer bc.BlockMutex.RUnlock()

    // TODO:: check if the t == getTByUUID(t.UUID)
    if blocks, ok := bc.Trans2Blocks[t.UUID]; ok {
        for _, block := range blocks {
            if block.Valid && block.Block.BlockID >= bc.LatestBlock.Block.BlockID - 6 {
                return 2, block.Hash
            }
        }
        return 1, blocks[0].Hash
    }

    return 0, "?"
}

func (bc *BlockChain) GetUserInfo(uid string) (u *UserInfo) {
    return bc.setDefaultUserInfo(uid)
}

func (bc *BlockChain) GetUserInfoWithDefault(uid string) (u *UserInfo) {
    u, ok := bc.getUserInfo(uid)
    if !ok {
        return bc.defaultUserInfo
   }
   return u
}

// Private: Hash

func (bc *BlockChain) getHashStringOfBlock(b *pb.Block) (s string, err error) {
    jsonString, err := bc.jsonMarshaler.MarshalToString(b)
    if err != nil {
        return
    }

    s = GetHashString(jsonString)
    return
}

// Private: Block

func (bc *BlockChain) refreshBlockChain(bi *BlockInfo) (latestChanged bool, err error) {
    bc.Blocks[bi.Hash] = bi

    // Handle chain switch and go through the transactions in `b`
    // TODO:: done
    b := bi.Block
    for _, trans := range b.Transactions {
        blocks := bc.Trans2Blocks[trans.UUID]
        if blocks == nil {
            blocks = make([]*BlockInfo, 0)
        }
        blocks = append(blocks, bi)
        bc.Trans2Blocks[trans.UUID] = blocks
    }

    height := bc.LatestBlock.Block.BlockID
    latestChanged = b.BlockID > height || b.BlockID == height && bi.Hash < bc.LatestBlock.Hash
    if !latestChanged {
        return
    }

    if b.BlockID == height + 1 && b.PrevHash == bc.LatestBlock.Hash {
        // Extend, speed up
        bc.doBlock(bi)
    } else {
        bc.resetLatestBlock(bc.LatestBlock, bi)
    }
    bc.LatestBlock = bi
    return 
}

func (bc *BlockChain) doBlock(x *BlockInfo) {
    x.Valid = true
    for _, trans := range x.Block.Transactions {
        bc.doTransaction(trans)
    }
}

func (bc *BlockChain) undoBlock(x *BlockInfo) {
    x.Valid = false
    s := x.Block.Transactions
    for i := len(s) - 1; i >= 0; i -- {
        bc.undoTransaction(s[i])
    }
}

func (bc *BlockChain) resetLatestBlock(x *BlockInfo, y *BlockInfo) {
    // TODO: delete invalid block
    z := bc.findBlockLCA(x, y)
    for x != z {
        bc.undoBlock(x)
        x = bc.Blocks[x.Block.PrevHash]
    }
    a := make([]*BlockInfo, 0)
    for y != z {
        a = append(a, y)
        y = bc.Blocks[y.Block.PrevHash]
    }
    for i := len(a) - 1; i >= 0; i -- {
        bc.doBlock(a[i])
    }
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

func (bc *BlockChain) verifyBlock(bi *BlockInfo) (err error) {
    // We only verify basic info here (do not check whether the transaction itself is valid or not).
    // Return nil when succeed.
    if !CheckHash(bi.Hash) {
        return fmt.Errorf("Verify block failed, invalid hash: %s.", bi.Hash)
    }
    b := bi.Block

    // check used transaction
    for _, t := range b.Transactions {
        if blocks, ok := bc.Trans2Blocks[t.UUID]; ok {
            for _, block := range blocks {
                if block.Valid {
                    return fmt.Errorf("Verify block failed, transaction exists: %s.", t.UUID)
                }
            }
        }
    }

    // check validity
    stack := NewBlockChainTStack(bc, false)
    for _, t := range b.Transactions {
        if !stack.TestAndDo(t) {
            return fmt.Errorf("Verify block failed, transaction amount invalid: %s.", t.UUID)
        }
    }

    // TODO:: check previous block is indeed a block & recover

    return nil
}

// Private: Transaction

func (bc *BlockChain) verifyTransactionInfo(t *pb.Transaction) (err error) {
    // Return nil when succeed.
    // TODO:: done
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
        return fmt.Errorf("Verify transaction failed, insufficient value: %d, mining fee: %d.",
            t.Value, t.MiningFee)
    }
    // TODO:: if there is a transaction with same UUID, check it.

    return nil
}

func (bc *BlockChain) verifyTransactionMoney(t *pb.Transaction) (err error) {
    // Verify whether the transaction can be done (considering user accounts).
    // TODO:: done
    balance := bc.Users[t.FromID].Money
    if balance < t.Value {
        return fmt.Errorf("Verify transaction money failed, insufficient balance: %d, transfer money: %d.",
                        balance, t.Value)
    }           
    return nil
}

func (bc *BlockChain) doTransaction(t *pb.Transaction) (err error) {
    // TODO:: done
    bc.Users[t.FromID].Money -= t.Value
    bc.Users[t.ToID].Money += t.Value - t.MiningFee
    delete(bc.PendingTransactions, t.UUID)
    return nil
}

func (bc *BlockChain) undoTransaction(t *pb.Transaction) (err error) {
    // TODO:: done
    bc.Users[t.FromID].Money += t.Value
    bc.Users[t.ToID].Money -= t.Value - t.MiningFee
    bc.PendingTransactions[t.UUID] = t
    return nil
}

// Private: User

func (bc *BlockChain) getUserInfo(uid string) (u *UserInfo, ok bool) {
    bc.UserMutex.RLock()
    defer bc.UserMutex.RUnlock()

    u, ok = bc.Users[uid]
    return
}

func (bc *BlockChain) setDefaultUserInfo(uid string) (u *UserInfo) {
    bc.UserMutex.Lock()
    defer bc.UserMutex.Unlock()

    u, ok := bc.Users[uid]
    if !ok {
        u = &UserInfo{Money: bc.config.Common.DefaultMoney}
        bc.Users[uid] = u
    }
    return
}

// func (bc *BlockChain) triggerCallbacks(funcname string, ...param string) {
//     switch funcname {
//     case "ChangeLatest":
//         for _, c := range bc.OnChangeLatestCallbacks {
//             c()
//         }
//     case "AddActive":
//         for _, c := range bc.OnAddActiveCallbacks {
//             c(...param)
//         }
//     case "RemoveActive":
//         for _, c := range bc.OnRemoveActiveCallbacks {
//             c(...param)
//         }
//     }
// }
