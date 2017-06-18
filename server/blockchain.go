package main

import (
    "sync"
    "log"
    "fmt"
    "strings"
    "strconv"
    "bufio"
    "path"
    "os"

    pb "../protobuf/go"
    "github.com/golang/protobuf/jsonpb"
    "github.com/golang/protobuf/proto"
)

const VerifyBlockInfo_TIdentical_Thresh = 10

type UserInfo struct {
    Money int32
}

type BlockInfo struct {
    Hash string
    Json string
    Block *pb.Block

    //OnLongest: on the longest branch
    OnLongest bool
}

type BlockChain struct {
    // hash(jsonify(b)) -> bi
    Blocks       map[string]*BlockInfo
    LatestBlock *BlockInfo

    // t.UUID -> []*BlockInfo
    // NOTE:: Owned by BlockMutex
    Trans2Blocks map[string][]*BlockInfo

    // NOTE:: Owned by TransactionMutex
    PendingTransactions *PriorityTransactionPool
    AllTransactions map[string]*pb.Transaction

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

    inTransactionRecovery bool
    inBlockRecovery bool

    defaultUserInfo   *UserInfo
    transactionWriter *bufio.Writer
}

func NewBlockChain(c *ServerConfig, p2pc *P2PClient) (bc *BlockChain) {
    bc = &BlockChain{
        Blocks: make(map[string]*BlockInfo),
        LatestBlock: &BlockInfo{
            Json: "{}",
            Block: &pb.Block{BlockID: 0},
            Hash: strings.Repeat("0", 64),
            OnLongest: true,
        },
        Trans2Blocks: make(map[string][]*BlockInfo),

        Users: make(map[string]*UserInfo),

        PendingTransactions: NewPriorityTransactionPool(),
        AllTransactions: make(map[string]*pb.Transaction),

        config: c,
        p2pc: p2pc,

        BlockMutex: &sync.RWMutex{},
        UserMutex: &sync.RWMutex{},
        TransactionMutex: &sync.RWMutex{},
        inTransactionRecovery: false,
        inBlockRecovery: false,

        defaultUserInfo: &UserInfo{Money: c.Common.DefaultMoney},
    }

    bc.Blocks[bc.LatestBlock.Hash] = bc.LatestBlock
    bc.recover()
    return
}

func (bc *BlockChain) recover() {
    bc.inTransactionRecovery = true
    log.Printf("Start recovery.")

    func () {
        transactionFile, err := os.Open(bc.config.Self.TransactionFile)
        if err == nil {
            return
        }
        defer transactionFile.Close()
        transactionReader := bufio.NewReader(transactionFile)

        for {
            bytes, eof, err := CRCLoadStream(transactionReader)
            if eof || err != nil {
                return
            }

            t := &pb.Transaction{}
            err = proto.Unmarshal(bytes, t)
            if err != nil {
                continue
            }

            _ = bc.PushTransaction(t, true)
        }
    }()

    bc.inTransactionRecovery = false

    bc.inBlockRecovery = true

    func () {
        msg, err := CRCLoad(bc.config.Self.LatestBlockFile)
        if err != nil {
            return
        }
        json, err := CRCLoad(path.Join(bc.config.Self.BlockDirectory, msg + ".json"))
        if err != nil {
            return
        }

        _, _ = bc.PushBlockJson(json)
    }()

    bc.inBlockRecovery = false

    if _, err := os.Stat(bc.config.Self.TransactionFile); os.IsExist(err) {
        transactionFile, err := os.OpenFile(bc.config.Self.TransactionFile, os.O_APPEND|os.O_WRONLY, 0644)
        if err == nil {
            bc.transactionWriter = bufio.NewWriter(transactionFile)
        }
    } else {
        transactionFile, err := os.OpenFile(bc.config.Self.TransactionFile, os.O_CREATE|os.O_WRONLY, 0644)
        if err == nil {
            bc.transactionWriter = bufio.NewWriter(transactionFile)
        }
    }

    log.Printf("Recovery ends.")
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
        OnLongest: false,
    }

    log.Printf("Push block internal: BlockID=%d, MinerID=%s, Hash=%s.", bi.Block.BlockID, bi.Block.MinerID, bi.Hash)

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

    if _, ok := bc.Blocks[bi.Hash]; ok {
        return false, fmt.Errorf("Push block failed: block exist: %s.", bi.Hash)
    }

    bc.UserMutex.Lock()
    defer bc.UserMutex.Unlock()
    bc.TransactionMutex.Lock()
    defer bc.TransactionMutex.Unlock()

    return bc.refreshBlockChain(bi)
}

func (bc *BlockChain) PushTransaction(t *pb.Transaction, needVerifyInfo bool) bool {
    // Verify transaction based on current info
    // RC: 0=fail, 1=ok, 2=already-in-current-chain

    log.Printf("Push transaction: %s.", t.UUID)

    if (needVerifyInfo) {
        if err := bc.verifyTransactionInfo(t); err != nil {
            return false
        }
    }

    bc.BlockMutex.RLock()
    defer bc.BlockMutex.RUnlock()

    bc.TransactionMutex.Lock()
    defer bc.TransactionMutex.Unlock()

    // NOTE:: We still need to check this, since side-chain transactions may not appear in PendingTransactions.
    if err := bc.verifyTransactionExist(t); err != nil {
        return false
    }

    // NOTE:: We don't verify the money in the transaction here.
    // bc.UserMutex.RLock()
    // defer bc.UserMutex.RUnlock()

    // err := bc.verifyTransactionMoney(t)
    // if (err != nil) {
    //     return 0
    // }

    bc.AllTransactions[t.UUID] = t
    ok := bc.addPendingTransaction(t)
    return ok
}

func (bc *BlockChain) VerifyTransaction6(t *pb.Transaction) (rc int, hash string) {
    // Return return code and hash
    // Return code: 0=fail; 1=peding; 2=succeed.

    bc.BlockMutex.RLock()
    defer bc.BlockMutex.RUnlock()

    bc.TransactionMutex.RLock()
    defer bc.TransactionMutex.RUnlock()

    if err := bc.verifyTransactionUUID(t); err != nil {
        return 0, "?"
    }

    if blocks, ok := bc.Trans2Blocks[t.UUID]; ok {
        for _, block := range blocks {
            if block.OnLongest {
                if block.Block.BlockID <= bc.LatestBlock.Block.BlockID - 6 {
                    return 2, block.Hash
                } else {
                    return 1, block.Hash
                }
            }
        }
    }

    // the transactions in PendingTransactions
    if bc.PendingTransactions.Has(t) {
        err := bc.verifyTransactionMoney(t)
        if (err != nil) {
            return 0, "?"
        }
        return 1, "!"
    }

    // NOTE:: ignore the block in side-chain

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
    var totalFee int32 = 0
    x.OnLongest = true
    for _, trans := range x.Block.Transactions {
        bc.doTransaction(trans)
        totalFee += trans.MiningFee
    }

    // Mining fee is computed at last
    bc.setDefaultUserInfo(x.Block.MinerID).Money += totalFee
}

func (bc *BlockChain) undoBlock(x *BlockInfo) {
    var totalFee int32 = 0
    x.OnLongest = false
    s := x.Block.Transactions
    for i := len(s) - 1; i >= 0; i-- {
        bc.undoTransaction(s[i])
        totalFee += s[i].MiningFee
    }

    // Undo the mining fee.
    bc.setDefaultUserInfo(x.Block.MinerID).Money -= totalFee
}

func (bc *BlockChain) doTransaction(t *pb.Transaction) (err error) {
    // Require: UserMutex, TransactionMutex.
    bc.setDefaultUserInfo(t.FromID).Money -= t.Value
    bc.setDefaultUserInfo(t.ToID).Money += t.Value - t.MiningFee
    bc.PendingTransactions.Remove(t)
    return nil
}

func (bc *BlockChain) undoTransaction(t *pb.Transaction) (err error) {
    // Require: UserMutex, TransactionMutex.
    bc.setDefaultUserInfo(t.FromID).Money += t.Value
    bc.setDefaultUserInfo(t.ToID).Money -= t.Value - t.MiningFee
    bc.addPendingTransaction(t)
    return nil
}

// Private: Block

func (bc *BlockChain) addPendingTransaction(t *pb.Transaction) bool {
    if bc.PendingTransactions.Has(t) {
        return true
    }

    diskDone := make(chan bool, 1)
    if !bc.inTransactionRecovery {
        go func () {
            ok := func() bool {
                if bc.transactionWriter == nil {
                    return false
                }

                bytes, err := proto.Marshal(t)
                if err != nil {
                    return false
                }

                err = CRCSaveStream(bc.transactionWriter, bytes)
                if err != nil {
                    return false
                }
                err = bc.transactionWriter.Flush()
                if err != nil {
                    return false
                }

                return true
            }()

            diskDone <- ok
        }()
    }

    bc.PendingTransactions.Add(t)

    if !bc.inTransactionRecovery {
        ok := <-diskDone
        return ok
    }

    return true
}

func (bc *BlockChain) addBlock(bi *BlockInfo) {
    // Add a verified (info only, no transactions) into the database
    // Require BlockMutex
    // log.Printf("Add block: BlockID=%d, Hash=%s.\n", bi.Block.BlockID, bi.Hash)

    diskDone := make(chan bool, 1)
    if !bc.inTransactionRecovery {
        go func () {
            _ = CRCSave(path.Join(bc.config.Self.BlockDirectory, bi.Hash + ".json"), bi.Json)
            diskDone <- true
        }()
    }

    bc.Blocks[bi.Hash] = bi
    for _, t := range bi.Block.Transactions {
        blocks := bc.Trans2Blocks[t.UUID]
        if blocks == nil {
            blocks = make([]*BlockInfo, 0)
        }
        blocks = append(blocks, bi)
        bc.Trans2Blocks[t.UUID] = blocks
        bc.AllTransactions[t.UUID] = t
    }

    if !bc.inTransactionRecovery {
        <-diskDone
    }
}

func (bc *BlockChain) setLatestBlock(bi *BlockInfo) {
    diskDone := make(chan bool, 1)
    if !bc.inBlockRecovery {
        go func () {
            _ = CRCSave(path.Join(bc.config.Self.LatestBlockFile + ".tmp"), bi.Hash)
            _ = os.Rename(bc.config.Self.LatestBlockFile + ".tmp", bc.config.Self.LatestBlockFile)
            _ = os.Rename(bc.config.Self.LatestBlockFile + ".tmp.crc", bc.config.Self.LatestBlockFile + ".crc")

            diskDone <- true
        }()
    }

    bc.LatestBlock = bi

    if !bc.inBlockRecovery {
        <-diskDone
    }
}

func (bc *BlockChain) refreshBlockChain(bi *BlockInfo) (latestChanged bool, err error) {
    // Handle chain switch and go through the transactions in `b`
    bc.addBlock(bi)

    b := bi.Block
    height := bc.LatestBlock.Block.BlockID
    latestChanged = b.BlockID > height || b.BlockID == height && bi.Hash < bc.LatestBlock.Hash

    if latestChanged && bc.config.Miner.EnableSelfLatestCheating {
        if b.BlockID == height && b.MinerID != bc.config.Self.ID && bc.LatestBlock.Block.MinerID == bc.config.Self.ID {
            latestChanged = false
        }
    }

    if !latestChanged {
        log.Printf("!! LatestBlock change failed (pre-checking stage): %s.\n    Remains %s.", bi.Hash, bc.LatestBlock.Hash)
        return
    }

    if b.BlockID == height + 1 && b.PrevHash == bc.LatestBlock.Hash {
        latestChanged = bc.extendLatestBlock(bi)
    } else {
        latestChanged = bc.switchLatestBlock(bc.LatestBlock, bi)
    }

    if latestChanged {
        log.Printf("!! LatestBlock changed to: %s.", bc.LatestBlock.Hash)
    } else {
        log.Printf("!! LatestBlock change failed (execution stage): %s.\n    Remain %s.", bi.Hash, bc.LatestBlock.Hash)
    }

    return
}

func (bc *BlockChain) extendLatestBlock(bi *BlockInfo) (succ bool) {
    if err := bc.verifyBlockTransaction(bi); err == nil {
        bc.doBlock(bi)
        bc.setLatestBlock(bi)
        return true
    }
    return false
}

func (bc *BlockChain) switchLatestBlock(source *BlockInfo, target *BlockInfo) (succ bool) {
    x, y := source, target

    succ = bc.switchLatestBlock_complete(y)
    if !succ {
        return
    }

    z := bc.findBlockLCA(x, y)

    // NOTE:: Keep the invalid blocks.

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

    bc.setLatestBlock(target)
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

        if len(prev) == 0 || bi.OnLongest {
            break
        }

        if _, ok := bc.Blocks[prev]; ok {
            bi = bc.Blocks[prev]
            continue
        }

        done := func () bool {
            json, err := CRCLoad(path.Join(bc.config.Self.BlockDirectory, prev + ".json"))
            if err != nil {
                return false
            }

            block := &pb.Block{}
            err = jsonpb.UnmarshalString(json, block)

            if err != nil {
                return false
            }

            newBi := &BlockInfo{
                Json: json,
                Hash: GetHashString(json),
                Block: block,
                OnLongest: false,
            }

            if prev != newBi.Hash {
                return false
            }

            if err = bc.verifyBlockInfo(newBi); err == nil {
                log.Printf("Completion succeeded on: %s (DISK).", prev)
                bc.addBlock(newBi)
                return true
            }

            return false
        }()

        if !done {
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
                    OnLongest: false,
                }

                if prev != newBi.Hash {
                    continue
                }

                if err = bc.verifyBlockInfo(newBi); err == nil {
                    log.Printf("Completion succeeded on: %s (QUERY).", prev)
                    bc.addBlock(newBi)
                    response.AcquireClose()
                    break
                }
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
        return fmt.Errorf("Verify block failed, invalid hash: %s.", bi.Hash)
    }

    b := bi.Block
    // Check hex
    if !CheckNonce(b.Nonce) {
        return fmt.Errorf("Verify block failed, invalid nonce: %s.", b.Nonce)
    }

    if len(b.MinerID) != 8 || b.MinerID[0:6] != "Server" {
        return fmt.Errorf("Verify block failed, invalid miner ID: %s.", b.MinerID)
    } else if _, err = strconv.Atoi(b.MinerID[6:8]); err != nil {
        return
    }

    if len(b.Transactions) > 50 {
        return fmt.Errorf("Verify block failed, too many transactions: %d > 50.", len(b.Transactions))
    }

    if !bc.verifyBlockInfo_tidentical(bi) {
        return fmt.Errorf("Verify block failed, repeated transactions.")
    }

    // Check basic transaction info
    for _, t := range b.Transactions {
        if err = bc.verifyTransactionInfo(t); err != nil {
            return err
        }
    }

    return nil
}

func (bc *BlockChain) verifyBlockInfo_tidentical(bi *BlockInfo) bool {
    ts := bi.Block.Transactions

    if len(ts) <= VerifyBlockInfo_TIdentical_Thresh {
        n := len(ts)
        for i := 1; i < n; i++ {
            for j := 0; j < i; j++ {
                if ts[i].UUID == ts[j].UUID {
                    return false
                }
            }
        }
        return true
    } else {
        has := make(map[string]bool)
        for _, t := range ts {
            if _, ok := has[t.UUID]; ok {
                return false
            }
            has[t.UUID] = true
        }
        return true
    }
}

func (bc *BlockChain) verifyBlockTransaction(bi *BlockInfo) (err error) {
    // We don't need to only verify basic info here
    // Return nil when succeed.
    // Require: BlockMutex.R, TransactionMutex.R, UserMutex.R
    b := bi.Block
    prev := bc.Blocks[b.PrevHash]
    if b.BlockID != prev.Block.BlockID + 1 {
        return fmt.Errorf("Verify block failed, invalid BlockID: %d.", b.BlockID)
    }

    // Check used transaction
    for _, t := range b.Transactions {
        if err = bc.verifyTransactionUUID(t); err != nil {
            return err
        }
        if err = bc.verifyTransactionRepeat(t); err != nil {
            return err
        }
    }

    // Check OnLongestity
    stack := NewBlockChainTStack(bc, false, false)
    defer stack.Close()
    for _, t := range b.Transactions {
        if !stack.TestAndDo(t) {
            return fmt.Errorf("Verify block failed, transaction amount invalid: %s.", t.UUID)
        }
    }

    return nil
}

// Private: Transaction

func (bc *BlockChain) verifyTransactionInfo(t *pb.Transaction) (err error) {
    // Return nil when succeed.

    if t.Type != pb.Transaction_TRANSFER {
        return fmt.Errorf("Verify transaction failed, unsupported type: %s.", t.Type)
    }
    if len(t.FromID) != 8 {
        return fmt.Errorf("Verify transaction failed, invalid FromID: %s.", t.FromID)
    }
    if len(t.ToID) != 8 {
        return fmt.Errorf("Verify transaction failed, invalid ToID: %s.", t.ToID)
    }
    if t.FromID == t.ToID {
        return fmt.Errorf("Verify transaction failed, same FromID and ToID: %s, %s.", t.FromID, t.ToID)
    }
    if t.MiningFee <= 0 {
        return fmt.Errorf("Verify transaction failed, non-positive mining fee: %d.", t.MiningFee)
    }
    if t.Value < 0 {
        return fmt.Errorf("Verify transaction failed, negative value: %d.", t.Value)
    }
    if t.Value <= t.MiningFee {
        return fmt.Errorf("Verify transaction failed, insufficient value: %d, mining fee: %d.", t.Value, t.MiningFee)
    }

    return nil
}

func (bc *BlockChain) verifyTransactionUUID(t *pb.Transaction) (err error) {
    // Verify whether there is some transaction with same UUID but different value.
    // Return nil when succeed.
    // Require: TransactionMutex.R
    if f, ok := bc.AllTransactions[t.UUID]; ok {
        if f.Type != t.Type || f.FromID != t.FromID || f.ToID != t.ToID ||
           f.Value != t.Value || f.MiningFee != t.MiningFee {
            return fmt.Errorf("Verify transaction failed: different transaction with same UUID %s.", t.UUID)
        }
    }
    return nil
}

func (bc *BlockChain) verifyTransactionExist(t *pb.Transaction) (err error) {
    // Verify whether the transaction has already been received.
    // Return nil when succeed.
    // Require: BlockMutex.R, TransactionMutex.R

    if bc.PendingTransactions.Has(t) {
        return fmt.Errorf("Verify transaction failed, transaction exists in pending queue: %s.", t.UUID)
    }

    if err = bc.verifyTransactionRepeat(t); err != nil {
        return err
    }

    return nil
}

func (bc *BlockChain) verifyTransactionRepeat(t *pb.Transaction) (err error) {
    // Verify whether the transaction has already appeared on the current chain.
    // Return nil when succeed.
    // Require: BlockMutex.R

    if blocks, ok := bc.Trans2Blocks[t.UUID]; ok {
        for _, block := range blocks {
            if block.OnLongest {
                return fmt.Errorf("Verify transaction failed, transaction exists on longest: %s.", t.UUID)
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
