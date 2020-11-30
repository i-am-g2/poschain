package blockchain

import (
	"fmt"
	"log"
	"sync"
)

var Blockchain []Block

// Block implem
type Block struct {
	Index     int
	Timestamp string
	Trans     Transaction
	Hash      string
	PrevHash  string
}

type Transaction struct {
	CardID   int
	Amount   int
	SignerID string
}

func init() {
	var genesisBlock Block
	genesisBlock.Timestamp = "0"
	genesisBlock.PrevHash = ""
	genesisBlock.Index = 0
	genesisBlock.Trans = Transaction{0, 0, "0"}
	genesisBlock.PrevHash = ""
	genesisBlock.Hash = calculateHash(genesisBlock)
	Blockchain = append(Blockchain, genesisBlock)
	log.Println("Created Block")
}

func (tr *Transaction) AsString() string {
	return fmt.Sprintf("Card %d | Amount by %d | Pos %s", tr.CardID, tr.Amount, tr.SignerID)
}

func (br *Block) DumpToString() string {
	return fmt.Sprintf("Index %v | Hash %s | PrevHash %s | CardID %v | Amount %v  | PosID %v", br.Index, br.Hash, br.PrevHash, br.Trans.CardID, br.Trans.Amount, br.Trans.SignerID)
}

var Balance map[int]int

var mutex = &sync.Mutex{}
