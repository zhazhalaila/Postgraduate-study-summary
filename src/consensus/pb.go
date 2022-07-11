package consensus

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	merkletree "github.com/zhazhalaila/PipelineBFT/src/merkleTree"
	"github.com/zhazhalaila/PipelineBFT/src/message"
	"github.com/zhazhalaila/PipelineBFT/src/verify"
)

var (
	ErrPeerNotFound = errors.New("peer not found")
	ErrVerifyFail   = errors.New("signature verify fail")
)

type PB struct {
	// Global log
	logger    *log.Logger
	transport *libnet.NetworkTransport

	n                 int
	f                 int
	id                int
	epoch             int
	r                 int
	fromInitiator     int
	preparedThreshold int

	partialShard *[]byte
	rootHash     [32]byte
	branch       *[][32]byte
	// collect all voters' signatures, only initiator do this
	sigSets map[int][]byte

	// public key
	pubKey *secp256k1.PublicKey
	// private key
	priKey *secp256k1.PrivateKey
	// public key for all peers
	pubKeys map[int]*secp256k1.PublicKey

	// done
	done chan struct{}
	// stop pbc instance
	stop chan bool
	// read data from channel
	inCh chan message.PBEntrance
	// return result to epoch module
	epochEvent chan Event
}

func MakePB(logger *log.Logger,
	transport *libnet.NetworkTransport,
	n, f, id, epoch, r, fromInitiator int,
	pubKey *secp256k1.PublicKey,
	priKey *secp256k1.PrivateKey,
	pubKeys map[int]*secp256k1.PublicKey,
	epochEvent chan Event) *PB {
	pb := &PB{}
	pb.logger = logger
	pb.transport = transport
	pb.n = n
	pb.f = f
	pb.id = id
	pb.epoch = epoch
	pb.r = r
	pb.fromInitiator = fromInitiator
	pb.preparedThreshold = 2*pb.f + 1
	pb.sigSets = make(map[int][]byte)
	pb.pubKey = pubKey
	pb.priKey = priKey
	pb.pubKeys = pubKeys
	pb.done = make(chan struct{})
	pb.stop = make(chan bool)
	pb.inCh = make(chan message.PBEntrance, pb.n*pb.n)
	pb.epochEvent = epochEvent
	go pb.run()
	return pb
}

func (pb *PB) Input(msg message.PBEntrance) {
	pb.inCh <- msg
}

func (pb *PB) Stop() {
	close(pb.stop)
}

func (pb *PB) run() {
	defer func() {
		pb.done <- struct{}{}
	}()

	for {
		select {
		case <-pb.stop:
			return
		case msg := <-pb.inCh:
			pb.handleCommand(msg)
		}
	}
}

func (pb *PB) handleCommand(entrance message.PBEntrance) {
	// pb.logger.Println("PB: ", string(entrance.Payload))
	switch entrance.SpecificType {
	case message.NewTransactionsType:
		var newTxs message.NewTransaction
		err := json.Unmarshal(entrance.Payload, &newTxs)
		if err != nil {
			return
		}
		pb.handleNewTransaction(newTxs)

	case message.SendType:
		var send message.SEND
		err := json.Unmarshal(entrance.Payload, &send)
		if err != nil {
			return
		}
		pb.handleSend(send)

	case message.AckType:
		var ack message.ACK
		err := json.Unmarshal(entrance.Payload, &ack)
		if err != nil {
			return
		}
		pb.handleAck(ack)

	case message.DoneType:
		var done message.DONE
		err := json.Unmarshal(entrance.Payload, &done)
		if err != nil {
			return
		}
		pb.handleDone(done)

	default:
		pb.logger.Println("Unkonwn Type")
	}
}

// Initiator broadcast transaction to all nodes
func (pb *PB) handleNewTransaction(newTxs message.NewTransaction) {
	pb.logger.Printf("[Epoch:%d] [Round:%d] [Initiator:%d] receive txs from client.\n", pb.epoch, pb.r, pb.id)

	// Marshal transaction to bytes
	txsBytes, err := json.Marshal(newTxs.Transactions)
	if err != nil {
		return
	}

	// Erasure code schema
	shards, err := ECEncode(pb.f+1, pb.n-(pb.f+1), txsBytes)
	if err != nil {
		return
	}

	pb.logger.Println(shards)

	// Generate merkletree for shards
	mt, err := merkletree.MakeMerkleTree(shards)
	if err != nil {
		return
	}

	// Get merkletree root
	rootHash := mt[1]
	pb.rootHash = rootHash

	// Sign merkletree root
	signature, err := pb.priKey.Sign(rootHash[:])
	if err != nil {
		pb.logger.Printf("[Epoch:%d] [Round:%d] [Initiator:%d] generate signature for txs failed.\n",
			pb.epoch, pb.r, pb.id)
	}

	// Broadcast shards to all nodes
	for i := 0; i < pb.n; i++ {
		branch := merkletree.GetMerkleBranch(i, mt)

		send := message.SEND{
			Initiator:         pb.id,
			RootHash:          rootHash,
			RootHashSignature: signature.Serialize(),
			Branch:            &branch,
			Share:             &shards[i],
		}
		sendJson, _ := json.Marshal(send)

		pbEntrance := message.GenPBEntrance(message.SendType, pb.r, pb.id, sendJson)
		pbEntrancsJson, _ := json.Marshal(pbEntrance)

		entrance := message.GenEntrance(message.PBType, pb.epoch, pbEntrancsJson)

		select {
		case <-pb.stop:
			return
		default:
			pb.transport.SendToPeer(i, entrance)
		}
	}
}

func (pb *PB) handleSend(send message.SEND) {
	pb.logger.Println(send.Share)

	// If initiator is not excecept initiator, return
	if send.Initiator != pb.fromInitiator {
		pb.logger.Printf("[Epoch:%d] [Round:%d] [Instance:%d] Get proposer = %d, want = %d.\n",
			pb.epoch, pb.r, pb.fromInitiator, send.Initiator, pb.fromInitiator)
		return
	}

	// If receive invalid merkle tree node or receive redundant SEND from initiator, return
	if !merkletree.MerkleTreeVerify(*send.Share, send.RootHash, *send.Branch, pb.id) || (pb.partialShard != nil) {
		return
	}

	err := verify.VerifySignature(send.RootHash[:], send.RootHashSignature, pb.pubKeys, send.Initiator)
	if err != nil {
		pb.logger.Printf("[Epoch:%d] [Round:%d] Receive invalid SEND msg from [Initiator:%d].\n",
			pb.epoch, pb.r, pb.fromInitiator)
		return
	}

	voteSignature, err := pb.priKey.Sign(send.RootHash[:])
	if err != nil {
		pb.logger.Println(err)
		return
	}

	pb.logger.Printf("[Epoch:%d] [Round:%d] [Peer:%d] Receive valid SEND msg from [Initiator:%d].\n",
		pb.epoch, pb.r, pb.id, send.Initiator)

	pb.partialShard = send.Share
	pb.rootHash = send.RootHash
	pb.branch = send.Branch

	ack := message.ACK{
		RootHash:      send.RootHash,
		VoteSignature: voteSignature.Serialize(),
		Voter:         pb.id,
	}
	acsJson, _ := json.Marshal(ack)

	pbEntrance := message.GenPBEntrance(message.AckType, pb.r, pb.fromInitiator, acsJson)
	pbEntranceJson, _ := json.Marshal(pbEntrance)

	entrance := message.GenEntrance(message.PBType, pb.epoch, pbEntranceJson)

	// Send ACK to initiator
	select {
	case <-pb.stop:
		return
	default:
		pb.transport.SendToPeer(pb.fromInitiator, entrance)
	}
}

// Only initiator do this
func (pb *PB) handleAck(ack message.ACK) {
	// If ack.rootHash != pb.rootHash, return.
	if !bytes.Equal(ack.RootHash[:], pb.rootHash[:]) {
		return
	}

	// If receive redundant ACK, return
	if _, ok := pb.sigSets[ack.Voter]; ok {
		pb.logger.Printf("[Epoch:%d] [Round:%d] Receive redundant ACK msg from [Voter:%d].\n",
			pb.epoch, pb.r, ack.Voter)
		return
	}

	err := verify.VerifySignature(ack.RootHash[:], ack.VoteSignature, pb.pubKeys, ack.Voter)
	if err != nil {
		pb.logger.Printf("[Epoch:%d] [Round:%d] Receive invalid ACK msg from [Voter:%d].\n",
			pb.epoch, pb.r, ack.Voter)
		return
	}

	pb.logger.Printf("[Epoch:%d] [Round:%d] [Peer:%d] Receive valid ACK msg from [Voter:%d].\n",
		pb.epoch, pb.r, pb.id, ack.Voter)

	pb.sigSets[ack.Voter] = ack.VoteSignature

	// If acquire 2f+1 ACK, broadcast DONE
	if len(pb.sigSets) == 2*pb.f+1 {
		done := message.DONE{
			Initiator:  pb.id,
			RootHash:   pb.rootHash,
			Signatures: pb.sigSets,
		}
		doneJson, _ := json.Marshal(done)

		pbEntrance := message.GenPBEntrance(message.DoneType, pb.r, pb.id, doneJson)
		pbEntranceJson, _ := json.Marshal(pbEntrance)

		entrance := message.GenEntrance(message.PBType, pb.epoch, pbEntranceJson)

		select {
		case <-pb.stop:
			return
		default:
			pb.transport.Broadcast(entrance)
		}

		pb.logger.Printf("[Epoch:%d] [Round:%d] [Initiator:%d] Broadcast QC.\n",
			pb.epoch, pb.r, pb.id)
	}
}

func (pb *PB) handleDone(done message.DONE) {
	if done.Initiator != pb.fromInitiator {
		return
	}

	for voter, sig := range done.Signatures {
		err := verify.VerifySignature(done.RootHash[:], sig, pb.pubKeys, voter)
		if err != nil {
			pb.logger.Printf("[Epoch:%d] [Round:%d] Receive invalid DONE msg from [Initiator:%d] caused by [Voter:%d].\n",
				pb.epoch, pb.r, pb.fromInitiator, voter)
			return
		}
	}

	pb.logger.Printf("[Epoch:%d] [Round:%d] deliver PB instance [Initiator:%d].\n", pb.epoch, pb.r, pb.fromInitiator)

	select {
	case <-pb.stop:
		return
	default:
		qc := message.QuorumCert{
			Initiator:  pb.fromInitiator,
			Round:      pb.r,
			RootHash:   pb.rootHash,
			Signatures: done.Signatures,
		}

		pbOut := PBOutput{qc: qc}

		pb.epochEvent <- Event{
			eventType: DeliverPB,
			payload:   pbOut,
		}
	}
}
