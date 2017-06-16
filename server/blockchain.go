package main

import (
    "sync"

    pb "../protobuf/go"
    "github.com/golang/protobuf/jsonpb"
)

type UserInfo struct {
    Money int32
}

type BlockChain struct {
    // hash(jsonify(b)) -> b
    Blocks       map[string]*pb.Block
    // t.UUID -> t
    Transactions map[string]*pb.Transaction
    // UserID -> money
    Users        map[string]*UserInfo

    // Tail block of the longest block chain
    Latest     *pb.Block
    LatestHash string

    config            *ServerConfig
    blocksMutex       *sync.RWMutex
    transactionsMutex *sync.RWMutex
    usersMutex        *sync.RWMutex
    jsonMarshaller    *jsonpb.Marshaler
}

func NewBlockChain(c *ServerConfig) (bc *BlockChain) {
    return &BlockChain{
        Blocks: make(map[string]*pb.Block),
        Transactions: make(map[string]*pb.Transaction),
        Users: make(map[string]*UserInfo),
        Latest: nil,
        jsonMarshaller: &jsonpb.Marshaler{EnumsAsInts: false},
    }
}

// Public methods: block

func (bc *BlockChain) GetBlock(hash string) (b *pb.Block, err error) {
    bc.blocksMutex.RLock()
    defer bc.blocksMutex.RUnlock()
    // TODO::
    b = bc.Blocks[hash]
    return
}

func (bc *BlockChain) GetLatestBlock() (hash string, b *pb.Block, err error) {
    bc.blocksMutex.RLock()
    defer bc.blocksMutex.RUnlock()
    return bc.LatestHash, bc.Latest, err
}

func (bc *BlockChain) PushBlock(b *pb.Block, needVerify bool) (err error) {
    // Return nil when success
    if (needVerify) {
        err = bc.verifyBlock(b)
        if (err != nil) {
            return
        }
    }

    err = bc.refreshBlockChain(b)
    return
}

func (bc *BlockChain) DeclareNewBlock(b *pb.Block) (err error) {
    // Remove related transactions
    // TODO(MJY):: Is this implementation correct?
    return bc.PushBlock(b, false)
}

func (bc *BlockChain) VerifyTransaction6(b *pb.Block) (rc int, err error) {
    // Return return code and err 
    // Return code: 0=; 1=; 2=.
    // TODO::
    return 2, nil
}

func (bc *BlockChain) PushTransaction(t *pb.Transaction, needVerify bool) (err error) {
    bc.transactionsMutex.Lock()
    defer bc.transactionsMutex.Unlock()
    // TODO::
    return nil
}

func (bc *BlockChain) RemoveTransaction(tid string) (err error) {
    bc.transactionsMutex.Lock()
    defer bc.transactionsMutex.Unlock()
    // TODO::
    return nil
}

func (bc *BlockChain) GetUserInfo(uid string) (u *UserInfo) {
    return bc.setDefaultUserInfo(uid)
}

// Private: Hash

func (bc *BlockChain) getHashStringOfBlock(b *pb.Block) (s string, err error) {
    jsonString, err := bc.jsonMarshaller.MarshalToString(b)
    if err != nil {
        return
    }

    s = GetHashString(jsonString)
    return
}

// Private: Block

func (bc *BlockChain) refreshBlockChain(b *pb.Block) (err error) {
    bc.blocksMutex.Lock()
    defer bc.blocksMutex.Unlock()

    hash, err := bc.getHashStringOfBlock(b)
    if err != nil {
        return err
    }
    bc.Blocks[hash] = b

    // Handle chain switch and go through the transactions in `b`
    bc.usersMutex.Lock()
    defer bc.usersMutex.Unlock()
    // TODO::

    bc.LatestHash = hash
    bc.Latest = b
    return
}

func (bc *BlockChain) verifyBlock(b *pb.Block) (err error) {
    // Return nil when success
    // TODO::
    return nil
}

// Private: User

func (bc *BlockChain) getUserInfoRaw(uid string) (u *UserInfo, err error) {
    bc.usersMutex.RLock()
    defer bc.usersMutex.RUnlock()
    return bc.getUserInfoAtomic(uid)
}

func (bc *BlockChain) setDefaultUserInfo(uid string) (u *UserInfo) {
    bc.usersMutex.Lock()
    defer bc.usersMutex.Unlock()

    u, err := bc.getUserInfoAtomic(uid)
    if err != nil {
        u = &UserInfo{Money: bc.config.Common.DefaultMoney}
        bc.Users[uid] = u
    }
    return
}

func (bc *BlockChain) getUserInfoAtomic(uid string) (u *UserInfo, err error) {
    u = bc.Users[uid]
    return u, nil
}

