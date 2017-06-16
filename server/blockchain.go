package main

import (
    "sync"
    "fmt"

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
    // hash(jsonify(b)) -> b
    Blocks       map[string]*BlockInfo
    // t.UUID -> t
    Transactions map[string]*pb.Transaction
    // UserID -> money
    Users        map[string]*UserInfo

    // Tail block of the longest block chain
    Latest      *BlockInfo

    config            *ServerConfig
    p2pc              *P2PClient

    blocksMutex       *sync.RWMutex
    transactionsMutex *sync.RWMutex
    usersMutex        *sync.RWMutex

    jsonMarshaler     *jsonpb.Marshaler
    defaultUserInfo   *UserInfo
}

func NewBlockChain(c *ServerConfig, p2pc *P2PClient) (bc *BlockChain) {
    return &BlockChain{
        Blocks: make(map[string]*BlockInfo),
        Transactions: make(map[string]*pb.Transaction),
        Users: make(map[string]*UserInfo),
        Latest: nil,

        config: c,
        p2pc: p2pc,

        blocksMutex: &sync.RWMutex{},
        transactionsMutex: &sync.RWMutex{},
        usersMutex: &sync.RWMutex{},

        jsonMarshaler: &jsonpb.Marshaler{EnumsAsInts: false},
        defaultUserInfo: &UserInfo{Money: bc.config.Common.DefaultMoney},

    }
}

// Public methods: block

func (bc *BlockChain) GetBlock(hash string) (bi *BlockInfo, err error) {
    bc.blocksMutex.RLock()
    defer bc.blocksMutex.RUnlock()
    // TODO::
    bi = bc.Blocks[hash]
    return
}

func (bc *BlockChain) GetLatestBlock() (bi *BlockInfo) {
    bc.blocksMutex.RLock()
    defer bc.blocksMutex.RUnlock()
    return bc.Latest
}

func (bc *BlockChain) PushBlock(json string, needVerify bool) (err error) {
    block := &pb.Block{}
    jsonpb.UnmarshalString(json, block)

    bi := &BlockInfo{
        Json: json,
        Hash: GetHashString(json),
        Block: block,
    }

    // Return nil when succeed.
    if (needVerify) {
        err = bc.verifyBlock(bi)
        if (err != nil) {
            return
        }
    }

    return bc.refreshBlockChain(bi)
}

func (bc *BlockChain) DeclareNewBlock(json string) (err error) {
    // Remove related transactions.
    // TODO(MJY):: Is this implementation correct?
    return bc.PushBlock(json, false)
}

func (bc *BlockChain) VerifyTransaction6(t *pb.Transaction) (rc int, hash string) {
    // Return return code and err 
    // Return code: 0=fail; 1=peding; 2=success.
    // TODO::
    return 0, "?"
}

func (bc *BlockChain) PushTransaction(t *pb.Transaction, needVerify bool) (err error) {
    // Return nil when succeed.
    bc.transactionsMutex.Lock()
    defer bc.transactionsMutex.Unlock()
    // TODO:: done
    bc.Transactions[t.UUID] = t
    return nil
}

func (bc *BlockChain) RemoveTransaction(tid string) (err error) {
    // Return nil when succeed.
    bc.transactionsMutex.Lock()
    defer bc.transactionsMutex.Unlock()
    // TODO:: done
    delete(bc.Transactions, tid)
    return nil
}

func (bc *BlockChain) GetUserInfo(uid string) (u *UserInfo) {
    return bc.setDefaultUserInfo(uid)
}

func (bc *BlockChain) GetUserInfoWithDefault(uid string) (u *UserInfo) {
    u = bc.getUserInfo(uid)
    if u == nil {
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

func (bc *BlockChain) refreshBlockChain(bi *BlockInfo) (err error) {
    bc.blocksMutex.Lock()
    defer bc.blocksMutex.Unlock()

    // hash, err := bc.getHashStringOfBlock(b)
    // if err != nil {
    //     return err
    // }
    bc.Blocks[bi.Hash] = bi

    // Handle chain switch and go through the transactions in `b`
    bc.usersMutex.Lock()
    defer bc.usersMutex.Unlock()
    // TODO::

    bc.Latest = bi
    return
}

func (bc *BlockChain) verifyBlock(bi *BlockInfo) (err error) {
    // Return nil when success
    // TODO:: partial done
    hash, err := bc.getHashStringOfBlock(bi.Block)
    if err != nil {
        return err
    }
    succ := CheckHash(hash)
    if succ == false {
        return fmt.Errorf("Verify block failed, invalid hash: %s.", hash)
    }
    // TODO::
    return nil
}

// Private: User

func (bc *BlockChain) getUserInfo(uid string) (u *UserInfo) {
    bc.usersMutex.RLock()
    defer bc.usersMutex.RUnlock()
    return bc.Users[uid]
}

func (bc *BlockChain) setDefaultUserInfo(uid string) (u *UserInfo) {
    bc.usersMutex.Lock()
    defer bc.usersMutex.Unlock()

    u = bc.Users[uid]
    if u == nil {
        u = &UserInfo{Money: bc.config.Common.DefaultMoney}
        bc.Users[uid] = u
    }
    return
}

