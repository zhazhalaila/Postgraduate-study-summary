package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/zhazhalaila/PipelineBFT/src/fake"
	"github.com/zhazhalaila/PipelineBFT/src/message"
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
	for i := 0; i < *n; i++ {
		conn, err := net.Dial("tcp", "127.0.0.1:800"+strconv.Itoa(i))
		if err != nil {
			log.Fatal(err)
		}

		w := bufio.NewWriterSize(conn, 4096)
		enc := json.NewEncoder(w)

		remoteConns[i] = remoteConn{conn: conn, w: w, enc: enc}
	}

	for max := 0; max < 200; max++ {
		for r := 0; r < *k; r++ {
			for i := 0; i < *n; i++ {
				// Create new transaction request
				req := message.NewTransaction{
					ClientAddr:   remoteConns[i].conn.LocalAddr().String(),
					Transactions: fake.FakeBatchTx(1, 1, r, i),
				}
				reqJson, _ := json.Marshal(req)

				pbEntrance := message.GenPBEntrance(message.NewTransactionsType, -1, -1, reqJson)
				pbEntranceJson, _ := json.Marshal(pbEntrance)

				entrance := message.GenEntrance(message.PBType, -1, pbEntranceJson)

				writedata(remoteConns[i].w, remoteConns[i].enc, entrance)

			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	for i := 0; i < *n; i++ {
		remoteConns[i].conn.Close()
	}
}
