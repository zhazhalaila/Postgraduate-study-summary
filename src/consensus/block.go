package consensus

import "sync"

type block struct {
	txs map[int]map[int]*[][]byte
}

type ChainBlock struct {
	mu     sync.Mutex
	blocks map[int]block
}

// Create new chained block, no folk
func MakeChainBlock() *ChainBlock {
	cb := &ChainBlock{}
	cb.blocks = make(map[int]block)
	return cb
}

// Get block for epoch
func (cb *ChainBlock) GetBlock(epoch int) (bool, *block) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if b, ok := cb.blocks[epoch]; ok {
		return true, &b
	}

	return false, nil
}

// Append block to chained blocks
func (cb *ChainBlock) AppendBlock(epoch int, txs map[int]map[int]*[][]byte) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if _, ok := cb.blocks[epoch]; !ok {
		return
	}

	cb.blocks[epoch] = block{txs: txs}
}
