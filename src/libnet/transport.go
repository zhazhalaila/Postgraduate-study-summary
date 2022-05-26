package libnet

import (
	"bufio"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net"
)

const (
	bufSize = math.MaxUint16
)

var (
	// ErrTransportStop is returned when stop channel is closed
	ErrTransportStop = errors.New("Transport shutdown")
)

type NetworkTransport struct {
	logger *log.Logger

	port     string
	listener net.Listener

	logInCh  chan remoteConn
	logOutCh chan string

	peerRegisterCh chan peer

	// Send data to consensus module
	consumeCh chan interface{}
	// Read data from consensus module and broadcast data to all peers
	peerBroadcastCh chan interface{}
	// Read consens result from consensus module
	resultCh  chan interface{}
	stopCh    chan struct{}
	releaseCh chan struct{}
}

type remoteConn struct {
	remoteAddr string
	enc        *json.Encoder
}

type peer struct {
	id   int
	conn net.Conn
	w    *bufio.Writer
	enc  *json.Encoder
}

func NewNetworkTransport(logger *log.Logger, port string) *NetworkTransport {
	nt := &NetworkTransport{}
	nt.logger = logger
	nt.port = port
	nt.logInCh = make(chan remoteConn, 100)
	nt.logOutCh = make(chan string, 100)
	nt.peerRegisterCh = make(chan peer, 100)
	nt.consumeCh = make(chan interface{}, 10000)
	nt.peerBroadcastCh = make(chan interface{}, 10000)
	nt.stopCh = make(chan struct{})
	nt.releaseCh = make(chan struct{})

	go nt.inComingManger()
	go nt.peerManger()

	return nt
}

// Start network
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

// inComing manger mange client state and send response to client
func (nt *NetworkTransport) inComingManger() {
	inConns := make(map[string]remoteConn)
	for {
		select {
		case <-nt.stopCh:
			return

		case client := <-nt.logInCh:
			if _, ok := inConns[client.remoteAddr]; !ok {
				inConns[client.remoteAddr] = client
			}

		case remoteAddr := <-nt.logOutCh:
			delete(inConns, remoteAddr)

		case msg := <-nt.resultCh:
			inConns[msg.(string)].enc.Encode(msg)

		default:
			continue
		}
	}
}

// peer manager manage peer state and send data to peer
func (nt *NetworkTransport) peerManger() {
	peers := make(map[int]peer)
	for {
		select {
		case <-nt.stopCh:
			return

		case peer := <-nt.peerRegisterCh:
			if _, ok := peers[peer.id]; !ok {
				peers[peer.id] = peer
			}

		case msg := <-nt.peerBroadcastCh:
			for id, peer := range peers {
				if err := peer.enc.Encode(msg); err != nil {
					nt.logger.Printf("Marshal data (send to peer:%d) error: %s.\n", id, err.Error())
				}
				if err := peer.w.Flush(); err != nil {
					nt.logger.Printf("Send data to peer:%d error: %s.\n", id, err.Error())
				}
			}
		}

	}
}

func (nt *NetworkTransport) handleConn(conn net.Conn) {
	defer func() {
		nt.logOutCh <- conn.RemoteAddr().String()
		conn.Close()
	}()

	r := bufio.NewReaderSize(conn, bufSize)
	w := bufio.NewWriterSize(conn, bufSize)
	dec := json.NewDecoder(r)
	enc := json.NewEncoder(w)

	select {
	case <-nt.stopCh:
		return
	case nt.logInCh <- remoteConn{remoteAddr: conn.RemoteAddr().String(), enc: enc}:
	}

	for {
		if err := nt.handleCommand(r, dec); err != nil {
			log.Println(err)
			return
		}
	}
}

func (nt *NetworkTransport) handleCommand(r *bufio.Reader, dec *json.Decoder) error {
	var req interface{}

	if err := dec.Decode(req); err != nil {
		return err
	}

	select {
	case <-nt.stopCh:
		return ErrTransportStop
	case nt.consumeCh <- req:
	}

	return nil
}
