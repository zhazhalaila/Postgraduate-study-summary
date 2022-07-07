package genhash

import (
	"crypto/sha256"
	"encoding/json"
)

type genCoin struct {
	Epoch int
	Round int
}

// Convert struct to byte
func ConvertStructToHashBytes(s interface{}) (*[32]byte, error) {
	converted, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	convertedHash := sha256.Sum256(converted)
	return &convertedHash, nil
}

// Get common coin
func CommmonCoin(epoch, round, n int) int {
	coinStruct := genCoin{
		Epoch: epoch,
		Round: round,
	}

	converted, err := json.Marshal(coinStruct)
	if err != nil {
		return 0
	}

	return int(sha256.Sum256(converted)[0]) % n
}
