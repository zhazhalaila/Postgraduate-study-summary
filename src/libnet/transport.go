package libnet

import (
	"bufio"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net"

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

	peerRegisterCh chan peer

	// Send data to consensus module
	consumeCh chan Request
	// Read data from consensus module and broadcast data to all peers
	outBroadcastCh chan broadcastData
	// Read data from consensus module and send data to specific peer
	outSingleCh chan peerData
	// Read consens result from consensus module and write result to client
	resultCh chan interface{}

	stopCh chan struct{}
}

type Request struct {
	Epoch int
	Msg   interface{}
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

type broadcastData struct {
	msgType uint8
	msg     interface{}
}

type peerData struct {
	peerId  int
	msgType uint8
	msg     interface{}
}

func NewNetworkTransport(logger *log.Logger, port string) *NetworkTransport {
	nt := &NetworkTransport{}
	nt.logger = logger
	nt.port = port
	nt.logInCh = make(chan remoteConn, 100)
	nt.logOutCh = make(chan string, 100)
	nt.peerRegisterCh = make(chan peer, 100)
	nt.consumeCh = make(chan Request, 10000)
	nt.outBroadcastCh = make(chan broadcastData, 10000)
	nt.outSingleCh = make(chan peerData, 10000)
	nt.resultCh = make(chan interface{}, 100)
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
func (nt *NetworkTransport) Consume() <-chan Request {
	return nt.consumeCh
}

// Consensu module will use stopped func to check network whether closed or not
func (nt *NetworkTransport) Stopped() <-chan struct{} {
	return nt.stopCh
}

// Broadcast data to all peers
func (nt *NetworkTransport) Broadcast(msgType uint8, msg interface{}) {
	select {
	case <-nt.stopCh:
		return
	case nt.outBroadcastCh <- broadcastData{msgType: msgType, msg: msg}:
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
		nt.peerRegisterCh <- peer{id: id, conn: conn, w: w, enc: enc}
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

		case broadcastMsg := <-nt.outBroadcastCh:
			for id, peer := range peers {
				if err := nt.writeDataToPeer(peer.w, peer.enc, broadcastMsg.msgType, broadcastMsg.msg); err != nil {
					nt.logger.Printf("Send data to [Peer:%d] error : %s.\n", id, err.Error())
					peer.conn.Close()
					delete(peers, id)
				}
			}

		case peerMsg := <-nt.outSingleCh:
			if peer, ok := peers[peerMsg.peerId]; ok {
				if err := nt.writeDataToPeer(peer.w, peer.enc, peerMsg.msgType, peerMsg.msg); err != nil {
					nt.logger.Printf("Send data to [Peer:%d] error : %s.\n", peerMsg.peerId, err.Error())
					peer.conn.Close()
					delete(peers, peerMsg.peerId)
				}
			}
		}
	}
}

// using Flush() func write data immediately
func (nt *NetworkTransport) writeDataToPeer(w *bufio.Writer,
	enc *json.Encoder,
	msgType uint8,
	msg interface{}) error {
	// Write msg type
	if err := w.WriteByte(msgType); err != nil {
		return err
	}

	// Send the msg
	if err := enc.Encode(msg); err != nil {
		return err
	}

	// Flush
	if err := w.Flush(); err != nil {
		return err
	}

	return nil
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
		if err := nt.handleCommand(r, dec); err != nil {
			log.Println(err)
			return
		}
	}
}

// handle command
func (nt *NetworkTransport) handleCommand(r *bufio.Reader, dec *json.Decoder) error {
	// Get the msg type
	msgType, err := r.ReadByte()
	if err != nil {
		return err
	}

	log.Println("Transport read type: ", msgType)

	var req Request

	switch msgType {
	case message.PreprepareType:
		var preprepare message.PrePrepare
		if err := dec.Decode(&preprepare); err != nil {
			return err
		}
		req.Epoch = preprepare.Epoch
		req.Msg = preprepare
	case message.PrepareType:
		var prepare message.Prepare
		if err := dec.Decode(&prepare); err != nil {
			return err
		}
		req.Epoch = prepare.Epoch
		req.Msg = prepare
	}

	log.Println("Tansaport: ", req)

	select {
	case <-nt.stopCh:
		return ErrTransportStop
	case nt.consumeCh <- req:
	}

	return nil
}
