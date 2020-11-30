package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"time"
)

func calculateHash(block Block) string {
	record := string(block.Index) + block.PrevHash + block.Trans.AsString()
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func generateBlock(oldBlock Block, transact Transaction) (Block, error) {
	var newBlock Block

	t := time.Now()
	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.Trans = transact
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = calculateHash(newBlock)

	return newBlock, nil
}

func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}

	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}

	if calculateHash(newBlock) != newBlock.Hash {
		return false
	}
	return true
}

func replaceChain(newBlocks []Block) {
	if len(newBlocks) > len(Blockchain) {
		Blockchain = newBlocks
	}
}

func getLastBlock() Block {
	lastIndex := len(Blockchain) - 1
	log.Printf("Accessing LastIndex %v", lastIndex)
	return Blockchain[lastIndex]
}

func validateAndAdd(block Block) {
	lastBlock := getLastBlock()
	log.Printf("Validation Started")
	if isBlockValid(block, lastBlock) {
		log.Printf("Validation Succ")
		Blockchain = append(Blockchain, block)
		log.Printf("Added Block | Index %v | Hash %s", block.Index, block.Hash)
		Balance[block.Trans.CardID] += block.Trans.Amount
	}
}

func generateNewBalanceMap() {
	Balance = make(map[int]int)

	for _, block := range Blockchain {
		Balance[block.Trans.CardID] += block.Trans.Amount
	}
}
