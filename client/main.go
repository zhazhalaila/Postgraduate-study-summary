package main

import (
	"bufio"
	"encoding/json"
	"log"
	"net"

	"github.com/zhazhalaila/PipelineBFT/src/fake"
	"github.com/zhazhalaila/PipelineBFT/src/message"
)

func writedata(w *bufio.Writer, enc *json.Encoder, msg interface{}) {
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
	// Create new connection
	conn, err := net.Dial("tcp", "127.0.0.1:8000")
	if err != nil {
		log.Fatal(err)
	}

	w := bufio.NewWriterSize(conn, 4096)
	enc := json.NewEncoder(w)

	// Create new transaction request
	req := message.NewTransaction{
		ClientAddr:   conn.LocalAddr().String(),
		Transactions: fake.FakeBatchTx(2, 1, 1, 0),
	}
	reqJson, _ := json.Marshal(req)

	pbEntrance := message.GenPBEntrance(message.NewTransactionsType, -1, -1, reqJson)
	pbEntranceJson, _ := json.Marshal(pbEntrance)

	entrance := message.GenEntrance(message.PBType, -1, pbEntranceJson)
	writedata(w, enc, entrance)

	// close connection
	conn.Close()
}
