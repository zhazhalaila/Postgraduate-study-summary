package genhash

import (
	"crypto/sha256"
	"encoding/json"
)

// Convert struct to byte
func ConvertStructToHashBytes(s interface{}) (*[32]byte, error) {
	converted, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	convertedHash := sha256.Sum256(converted)
	return &convertedHash, nil
}
