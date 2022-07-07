package verify

import (
	"errors"

	"github.com/decred/dcrd/dcrec/secp256k1"
)

var (
	ErrPeerNotFound = errors.New("peer not found")
	ErrVerifyFail   = errors.New("signature verify fail")
)

func VerifySignature(
	msgHash []byte,
	msgHashSignature []byte,
	pubKeys map[int]*secp256k1.PublicKey,
	sender int) error {
	signature, err := secp256k1.ParseDERSignature(msgHashSignature)
	if err != nil {
		return err
	}

	senderPubkey, ok := pubKeys[sender]
	if !ok {
		return ErrPeerNotFound
	}

	if verified := signature.Verify(msgHash, senderPubkey); !verified {
		return ErrVerifyFail
	}

	return nil
}
