package main

import (
	"bufio"
	"encoding/json"
	"log"
	"net"

	"github.com/zhazhalaila/PipelineBFT/src/fake"
	"github.com/zhazhalaila/PipelineBFT/src/message"
)

func writedata(w *bufio.Writer, enc *json.Encoder, msgType uint8, msg interface{}) {
	// Write request type
	if err := w.WriteByte(message.PreprepareType); err != nil {
		log.Fatal(err)
	}

	// Send the request
	if err := enc.Encode(msg); err != nil {
		log.Fatal(err)
	}

	// Flush
	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
}

func main() {
	// create new connection
	conn, err := net.Dial("tcp", "127.0.0.1:8000")
	if err != nil {
		log.Fatal(err)
	}

	w := bufio.NewWriterSize(conn, 4096)
	enc := json.NewEncoder(w)

	// create new message
	msg := message.NewTransaction{
		Epoch:        0,
		Round:        1,
		Initiator:    0,
		Transactions: fake.FakeBatchTx(2, 1, 1, 0),
	}

	writedata(w, enc, message.NewTransactionsType, msg)

	// close connection
	conn.Close()
}
