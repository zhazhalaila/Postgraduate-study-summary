package keys

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/decred/dcrd/dcrec/secp256k1"
)

type key struct {
	SigningKey   string
	VerifyingKey string
}

// decodeHex decodes the passed hex string and returns the resulting bytes.  It
// panics if an error occurs.  This is only used in the tests as a helper since
// the only way it can fail is if there is an error in the test source code.
func decodeHex(hexStr string) []byte {
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		panic("invalid hex string in test source: err " + err.Error() +
			", hex: " + hexStr)
	}

	return b
}

func readKey() ([]key, error) {
	// Read key-pair from file.
	data, err := ioutil.ReadFile("../replica.json")
	if err != nil {
		return nil, err
	}

	var keys []key
	err = json.Unmarshal(data, &keys)
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// Decode all peers' publickey
func DecodePublicKeys(n int) (map[int]*secp256k1.PublicKey, error) {
	publicKeys := make(map[int]*secp256k1.PublicKey, n)

	keys, err := readKey()
	if err != nil {
		return nil, err
	}
	keys = keys[:n]

	for i, key := range keys {
		_, pub := secp256k1.PrivKeyFromBytes(decodeHex(key.SigningKey))
		publicKeys[i] = pub
	}

	return publicKeys, nil
}

// Decode specific peer's key-pair
func DecodeKeyPair(peerId int) (*secp256k1.PrivateKey, *secp256k1.PublicKey, error) {
	keys, err := readKey()
	if err != nil {
		return nil, nil, err
	}

	fmt.Printf("[PeerId:%d] signingkey = %s.\n", peerId, keys[peerId].SigningKey)

	pri, pub := secp256k1.PrivKeyFromBytes(decodeHex(keys[peerId].SigningKey))
	return pri, pub, nil
}
