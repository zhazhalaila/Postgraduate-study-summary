package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"log"

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

func main() {
	data, err := ioutil.ReadFile("../replica.json")
	if err != nil {
		log.Fatal(err)
	}

	var keys []key
	err = json.Unmarshal(data, &keys)
	if err != nil {
		log.Fatal(err)
	}

	priv, pub := secp256k1.PrivKeyFromBytes(decodeHex(keys[0].SigningKey))

	hash := []byte{0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9}
	sig, err := priv.Sign(hash)
	if err != nil {
		log.Fatal(err)
	}

	if !sig.Verify(hash, pub) {
		log.Println("Could not verify.")
	} else {
		log.Println("Verify success")
	}

	serializedKey := priv.Serialize()
	if !bytes.Equal(serializedKey, decodeHex(keys[0].SigningKey)) {
		log.Println("Key not equal")
	} else {
		log.Println("Key equal")
	}
}
