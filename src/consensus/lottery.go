package consensus

import (
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	"github.com/zhazhalaila/PipelineBFT/src/message"
	"github.com/zhazhalaila/PipelineBFT/src/verify"
)

type Lottery struct {
	// Global log
	logger    *log.Logger
	transport *libnet.NetworkTransport

	n                int
	f                int
	id               int
	epoch            int
	fromLotteryTimes int

	// collect all coin shares
	coinShares map[int][]byte

	// public key
	pubKey *secp256k1.PublicKey
	// private key
	priKey *secp256k1.PrivateKey
	// public key for all peers
	pubKeys map[int]*secp256k1.PublicKey

	// done
	done chan struct{}
	// stop lottery instance
	stop chan bool
	// read data from channel
	inCh chan message.Lottery
	// return result to epoch module
	epochEvent chan Event
}

func MakeLottery(
	logger *log.Logger,
	transport *libnet.NetworkTransport,
	n, f, id, epoch, fromLotteryTimes int,
	pubKey *secp256k1.PublicKey,
	priKey *secp256k1.PrivateKey,
	pubKeys map[int]*secp256k1.PublicKey,
	epochEvent chan Event) *Lottery {
	l := &Lottery{}
	l.logger = logger
	l.transport = transport
	l.n = n
	l.f = f
	l.id = id
	l.epoch = epoch
	l.fromLotteryTimes = fromLotteryTimes
	l.coinShares = make(map[int][]byte)
	l.pubKey = pubKey
	l.priKey = priKey
	l.pubKeys = pubKeys
	l.done = make(chan struct{})
	l.stop = make(chan bool)
	l.inCh = make(chan message.Lottery, l.n)
	l.epochEvent = epochEvent
	go l.run()
	return l
}

func (l *Lottery) Input(msg message.Lottery) {
	l.inCh <- msg
}

func (l *Lottery) Stop() {
	close(l.stop)
}

func (l *Lottery) run() {
	defer func() {
		l.done <- struct{}{}
	}()

	for {
		select {
		case <-l.stop:
			return
		case msg := <-l.inCh:
			l.handleLottery(msg)
		}
	}
}

func (l *Lottery) handleLottery(lottery message.Lottery) {
	if lottery.LotteryTimes != l.fromLotteryTimes {
		return
	}

	if _, ok := l.coinShares[lottery.Sender]; ok {
		l.logger.Printf("[Epoch:%d] [LotteryTimes:%d] Receive redundant Coin share msg from [Sender:%d].\n",
			l.epoch, l.fromLotteryTimes, lottery.Sender)
		return
	}

	err := verify.VerifySignature(lottery.LotteryHash[:], lottery.LotterySignature, l.pubKeys, lottery.Sender)
	if err != nil {
		l.logger.Printf("[Epoch:%d] [LotteryTimes:%d] Receive invalid Coin share msg from [Sender:%d].\n",
			l.epoch, l.fromLotteryTimes, lottery.Sender)
		return
	}

	l.logger.Printf("[Epoch:%d] [LotteryTimes:%d] [Peer:%d] Receive valid Coin share msg from [Sender:%d].\n",
		l.epoch, l.fromLotteryTimes, l.id, lottery.Sender)
	l.coinShares[lottery.Sender] = lottery.LotterySignature

	if len(l.coinShares) == l.f+1 {
		select {
		case <-l.stop:
			return
		default:
			lotteryOut := LotteryOutput{
				lotteryTimes:   l.fromLotteryTimes,
				commonProducer: int(lottery.LotteryHash[0]) % l.n,
			}
			l.epochEvent <- Event{eventType: DeliverLottery, payload: lotteryOut}
		}
	}
}
