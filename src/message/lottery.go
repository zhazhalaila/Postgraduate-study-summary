package message

type Lottery struct {
	LotteryTimes     int
	Sender           int
	LotteryHash      [32]byte
	LotterySignature []byte
}

func GenLottery(lotteryTimes, sender int, lotteryHash [32]byte, lotterySignature []byte) Lottery {
	l := Lottery{
		LotteryTimes:     lotteryTimes,
		Sender:           sender,
		LotteryHash:      lotteryHash,
		LotterySignature: lotterySignature,
	}
	return l
}
