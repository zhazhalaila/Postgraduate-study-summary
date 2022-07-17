package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"log"
	"net"
	"time"

	"github.com/zhazhalaila/PipelineBFT/src/fake"
	"github.com/zhazhalaila/PipelineBFT/src/message"
	readaddress "github.com/zhazhalaila/PipelineBFT/src/readAddress"
)

type remoteConn struct {
	conn net.Conn
	w    *bufio.Writer
	enc  *json.Encoder
}

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
	n := flag.Int("n", 4, "total node number")
	k := flag.Int("k", 1, "maximum concurrent round for each epoch")
	flag.Parse()

	remoteConns := make(map[int]remoteConn, *n)
	addresses, err := readaddress.ReadAddress("../address.txt", *n)
	if err != nil {
		log.Fatal(err)
	}
	for id, addr := range addresses {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			log.Fatal(err)
		}

		w := bufio.NewWriterSize(conn, 4096)
		enc := json.NewEncoder(w)

		remoteConns[id] = remoteConn{conn: conn, w: w, enc: enc}
	}

	for max := 0; max < 10; max++ {
		for r := 0; r < *k; r++ {
			sendCh := make(chan struct{}, *n)
			for i := 0; i < *n; i++ {
				go func(r, i int) {
					// Create new transaction request
					req := message.NewTransaction{
						ClientAddr:   remoteConns[i].conn.LocalAddr().String(),
						Transactions: fake.FakeBatchTx(1024, 1, r, i),
					}
					reqJson, _ := json.Marshal(req)

					pbEntrance := message.GenPBEntrance(message.NewTransactionsType, -1, -1, reqJson)
					pbEntranceJson, _ := json.Marshal(pbEntrance)

					entrance := message.GenEntrance(message.PBType, -1, pbEntranceJson)

					writedata(remoteConns[i].w, remoteConns[i].enc, entrance)
					sendCh <- struct{}{}
				}(r, i)
			}
			for i := 0; i < *n; i++ {
				<-sendCh
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	for i := 0; i < *n; i++ {
		remoteConns[i].conn.Close()
	}
}
