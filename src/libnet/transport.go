package libnet

import (
	"bufio"
	"encoding/json"
	"log"
	"math"
	"net"

	"github.com/sasha-s/go-deadlock"
)

const (
	bufSize = math.MaxUint16
)

type NetworkTransport struct {
	logger *log.Logger

	mu deadlock.RWMutex

	port     string
	listener net.Listener
	peers    map[int]bufConn

	logInCh   chan client
	logOutCh  chan string
	consumeCh chan interface{}
	resultCh  chan interface{}
	stopCh    chan struct{}
	releaseCh chan struct{}
}

type client struct {
	remoteAddr string
	enc        *json.Encoder
	responseCh chan interface{}
}

type bufConn struct {
	conn net.Conn
	r    *bufio.Reader
	w    *bufio.Writer
	dec  *json.Decoder
	enc  *json.Encoder
}

func NewNetworkTransport(logger *log.Logger, port string) *NetworkTransport {
	nt := &NetworkTransport{}
	nt.logger = logger
	nt.port = port
	nt.peers = make(map[int]bufConn)
	nt.logInCh = make(chan client, 100)
	nt.logOutCh = make(chan string, 100)
	nt.consumeCh = make(chan interface{}, 10000)
	nt.stopCh = make(chan struct{})
	nt.releaseCh = make(chan struct{})

	go nt.clientManger()

	return nt
}

func (nt *NetworkTransport) Start() {
	var err error
	nt.listener, err = net.Listen("tcp", nt.port)

	if err != nil {
		nt.logger.Fatal(err)
	}

	nt.logger.Printf("Network port %s\n", nt.port)

	for {
		conn, err := nt.listener.Accept()
		if err != nil {
			select {
			case <-nt.stopCh:
				return
			default:
				nt.logger.Fatal("accept error: ", err)
			}
		}
		go nt.handleConn(conn)
	}
}

// clientManger mange client state and send response to client
func (nt *NetworkTransport) clientManger() {
	clients := make(map[string]client)
	for {
		select {
		case <-nt.stopCh:
			return

		case client := <-nt.logInCh:
			if _, ok := clients[client.remoteAddr]; ok {
				nt.logger.Printf("[Client:%s] has been registered.\n", client.remoteAddr)
				break
			}
			clients[client.remoteAddr] = client

		case remoteAddr := <-nt.logOutCh:
			delete(clients, remoteAddr)

		case msg := <-nt.resultCh:
			clients[msg.(string)].enc.Encode(msg)
		}
	}
}

func (nt *NetworkTransport) handleConn(conn net.Conn) {
	defer func() {
		// if peer close connection will cause no-op
		nt.logOutCh <- conn.RemoteAddr().String()
		conn.Close()
	}()

	// r := bufio.NewReaderSize(conn, bufSize)
	// w := bufio.NewWriterSize(conn, bufSize)
	// dec := json.NewDecoder(r)
	// enc := json.NewEncoder(w)

	// for {

	// }
}
