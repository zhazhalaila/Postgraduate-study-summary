package libnet

import (
	"bufio"
	"encoding/json"
	"errors"
	"log"
	"math"
	"math/rand"
	"net"
	"time"

	"github.com/zhazhalaila/PipelineBFT/src/message"
)

const (
	bufSize = math.MaxUint16
)

var (
	// ErrTransportStop is returned when stop channel is closed
	ErrTransportStop = errors.New("transport shutdown")
)

type NetworkTransport struct {
	logger *log.Logger

	port     string
	listener net.Listener

	logInCh  chan remoteConn
	logOutCh chan string

	peerRegisterCh   chan peer
	unRegisterPeerCh chan int

	// Send data to consensus module
	consumeCh chan message.Entrance
	// Read data from consensus module and broadcast data to all peers
	outBroadcastCh chan interface{}
	// Read data from consensus module and send data to specific peer
	outSingleCh chan peerData
	// Read consens result from consensus module and write result to client
	resultCh chan interface{}

	stopCh chan struct{}
}

type remoteConn struct {
	remoteAddr string
	enc        *json.Encoder
}

type peer struct {
	id    int
	conn  net.Conn
	w     *bufio.Writer
	enc   *json.Encoder
	outCh chan interface{}
}

type peerData struct {
	peerId int
	msg    interface{}
}

func NewNetworkTransport(logger *log.Logger, port string) *NetworkTransport {
	nt := &NetworkTransport{}
	nt.logger = logger
	nt.port = port
	nt.logInCh = make(chan remoteConn, 100)
	nt.logOutCh = make(chan string, 100)
	nt.peerRegisterCh = make(chan peer, 100)
	nt.unRegisterPeerCh = make(chan int, 100)
	nt.consumeCh = make(chan message.Entrance, 100000)
	nt.outBroadcastCh = make(chan interface{}, 100000)
	nt.outSingleCh = make(chan peerData, 100000)
	nt.resultCh = make(chan interface{}, 1000)
	nt.stopCh = make(chan struct{})

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
		nt.logger.Printf("Accept new connection from [%s].\n", conn.RemoteAddr().String())
		go nt.handleConn(conn)
	}
}

// Stop network
func (nt *NetworkTransport) Exit() {
	close(nt.stopCh)
}

// Read data from remote connection
func (nt *NetworkTransport) Consume() <-chan message.Entrance {
	return nt.consumeCh
}

// Consensu module will use stopped func to check network whether closed or not
func (nt *NetworkTransport) Stopped() <-chan struct{} {
	return nt.stopCh
}

// Broadcast data to all peers
func (nt *NetworkTransport) Broadcast(msg interface{}) {
	select {
	case <-nt.stopCh:
		return
	case nt.outBroadcastCh <- msg:
	}
}

// Send data to single peer
func (nt *NetworkTransport) SendToPeer(peerId int, msg interface{}) {
	select {
	case <-nt.stopCh:
		return
	case nt.outSingleCh <- peerData{peerId: peerId, msg: msg}:
	}
}

// Connect all remote peers
func (nt *NetworkTransport) ConnectAll(ipAddrs []string) error {
	for id, addr := range ipAddrs {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			nt.logger.Printf("Connect to [Peer:%d] failed.\n", id)
			return err
		}
		w := bufio.NewWriterSize(conn, bufSize)
		enc := json.NewEncoder(w)
		// Start a new goroutine to send data
		outCh := make(chan interface{}, 100000)
		go func(id int, w *bufio.Writer, enc *json.Encoder, outCh chan interface{}) {
			for msg := range outCh {
				// Network delay simulation (local server network delay is so low. e.g. under 2 ms)
				delayTime := rand.Intn(50-1) + 1
				time.Sleep(time.Duration(delayTime) * time.Millisecond)
				// Send the msg
				if err := enc.Encode(msg); err != nil {
					break
				}

				// Flush
				if err := w.Flush(); err != nil {
					break
				}
			}
			// If error, unregister peer
			select {
			case <-nt.stopCh:
				return
			default:
				nt.unRegisterPeerCh <- id
			}

		}(id, w, enc, outCh)
		nt.peerRegisterCh <- peer{id: id, conn: conn, w: w, enc: enc, outCh: outCh}
	}
	return nil
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
		}

		// Fix the problem of high CPU utilization
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

		case id := <-nt.unRegisterPeerCh:
			delete(peers, id)
			nt.logger.Printf("[Peer:%d] down.\n", id)

		case msg := <-nt.outBroadcastCh:
			for _, peer := range peers {
				peer.outCh <- msg
			}

		case peerMsg := <-nt.outSingleCh:
			if peer, ok := peers[peerMsg.peerId]; ok {
				peer.outCh <- peerMsg.msg
			}
		}
	}
}

// if new connect has been created cache it
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
		if err := nt.handleCommand(dec); err != nil {
			nt.logger.Println("Decode msg err: ", err)
			return
		}
	}
}

// handle command
func (nt *NetworkTransport) handleCommand(dec *json.Decoder) error {
	var entrance message.Entrance

	if err := dec.Decode(&entrance); err != nil {
		return err
	}

	// nt.logger.Println("Transport receive: , ", entrance, string(entrance.Payload))

	select {
	case <-nt.stopCh:
		return ErrTransportStop
	case nt.consumeCh <- entrance:
	}

	return nil
}
