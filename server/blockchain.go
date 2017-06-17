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
}

type BlockChain struct {
    // hash(jsonify(b)) -> bi
    Blocks       map[string]*BlockInfo
    LatestBlock *BlockInfo

    // t.UUID -> []*BlockInfo
    // NOTE:: Owned by blocksMutex
    Trans2Blocks map[string][]*BlockInfo

    // UserID -> money
    Users map[string]*UserInfo

    // Callbacks
    // OnChangeLatestCallbacks []func ()
    // OnAddActiveCallbacks []func (string)
    // OnRemoveActiveCallbacks []func (string)

    // NOTE:: Owned by blocksMutex
    isBlockOnLongest map[string]bool

    config            *ServerConfig
    p2pc              *P2PClient

    blocksMutex       *sync.RWMutex
    usersMutex        *sync.RWMutex

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
        },
        Trans2Blocks: make(map[string][]*BlockInfo),

        Users: make(map[string]*UserInfo),

        isBlockOnLongest: make(map[string]bool),

        config: c,
        p2pc: p2pc,

        blocksMutex: &sync.RWMutex{},
        usersMutex: &sync.RWMutex{},

        jsonMarshaler: &jsonpb.Marshaler{EnumsAsInts: false},
        defaultUserInfo: &UserInfo{Money: bc.config.Common.DefaultMoney},

        // OnChangeLatestCallbacks: make([]func (), 0)
        // OnAddActiveCallbacks: make([]func (string), 0)
        // OnRemoveActiveCallbacks: make([]func (string), 0)
    }
}

// Public methods: block

func (bc *BlockChain) GetBlock(hash string) (*BlockInfo, bool) {
    bc.blocksMutex.RLock()
    defer bc.blocksMutex.RUnlock()
    bi, ok := bc.Blocks[hash]
    return bi, ok
}

func (bc *BlockChain) GetLatestBlock() (bi *BlockInfo) {
    bc.blocksMutex.RLock()
    defer bc.blocksMutex.RUnlock()
    return bc.LatestBlock
}

func (bc *BlockChain) PushBlockJson(json string) (lastChanged bool, err error) {
    block := &pb.Block{}
    jsonpb.UnmarshalString(json, block)

    bi := &BlockInfo{
        Json: json,
        Hash: GetHashString(json),
        Block: block,
    }

    return bc.pushBlockInfo(bi, true, false)
}


func (bc *BlockChain) DeclareBlockInfo(bi *BlockInfo) (lastChanged bool, err error) {
    return bc.pushBlockInfo(bi, false, true)
}

func (bc *BlockChain) VerifyTransaction6(t *pb.Transaction) (rc int, hash string) {
    // Return return code and hash
    // Return code: 0=fail; 1=peding; 2=succeed..

    bc.blocksMutex.RLock()
    defer bc.blocksMutex.RUnlock()

    if blocks, ok := bc.Trans2Blocks[t.UUID]; ok {
        for _, block := range blocks {
            if bc.isBlockOnLongest[block.Hash] {
                // TODO::
                if true {
                    return 2, block.Hash
                } else {
                    return 1, block.Hash
                }
            }
        }
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

func (bc *BlockChain) refreshBlockChain(bi *BlockInfo, prioritySelectAsLatest bool) (latestChanged bool, err error) {
    bc.blocksMutex.Lock()
    defer bc.blocksMutex.Unlock()

    bc.Blocks[bi.Hash] = bi
    bc.LatestBlock = bi

    // Handle chain switch and go through the transactions in `b`
    bc.usersMutex.Lock()
    defer bc.usersMutex.Unlock()
    // TODO::
    return true, nil
}

func (bc *BlockChain) pushBlockInfo(bi *BlockInfo, needVerify bool, prioritySelectAsLatest bool) (lastChanged bool, err error) {
    lastChanged = false

    // Return nil when succeed.
    if (needVerify) {
        err = bc.verifyBlock(bi)
        if (err != nil) {
            return
        }
    }

    return bc.refreshBlockChain(bi, prioritySelectAsLatest)
}

func (bc *BlockChain) verifyBlock(bi *BlockInfo) (err error) {
    // We only verify basic info here (do not check whether the transaction itself is valid or not).
    // Return nil when succeed.
    succ := CheckHash(bi.Hash)
    if succ == false {
        return fmt.Errorf("Verify block failed, invalid hash: %s.", bi.Hash)
    }
    // TODO:: verify other info (incl. transactions).
    // verifyTransactionInfo(t)
    // verifyTransactionMoney(t)
    return nil
}

// Private: Transaction

func (bc *BlockChain) verifyTransactionInfo(t *pb.Transaction) (err error) {
    // Return nil when succeed.
    // TODO::
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
    return nil
}

// Private: User

func (bc *BlockChain) getUserInfo(uid string) (u *UserInfo, ok bool) {
    bc.usersMutex.RLock()
    defer bc.usersMutex.RUnlock()

    u, ok = bc.Users[uid]
    return
}

func (bc *BlockChain) setDefaultUserInfo(uid string) (u *UserInfo) {
    bc.usersMutex.Lock()
    defer bc.usersMutex.Unlock()

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
