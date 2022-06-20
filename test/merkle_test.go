package test

import (
	"log"
	"testing"

	"github.com/zhazhalaila/PipelineBFT/src/fake"
	merkletree "github.com/zhazhalaila/PipelineBFT/src/merkleTree"
)

func TestMerkleTree(t *testing.T) {
	txs := fake.FakeBatchTx(2, 1, 1, 1)
	mt, err := merkletree.MakeMerkleTree(txs)
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		branch := merkletree.GetMerkleBranch(i, mt)
		if ok := merkletree.MerkleTreeVerify((*txs)[i], mt[1], branch, i); !ok {
			t.Errorf("Verify = %t; want true", ok)
		}
	}
}
