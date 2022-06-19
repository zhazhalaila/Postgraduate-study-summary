package main

import (
	"flag"
	"log"
	"os"

	"github.com/zhazhalaila/PipelineBFT/src/consensus"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
)

func main() {
	logPath := flag.String("path", "log.txt", "log file path")
	port := flag.String("port", ":8000", "network port")
	n := flag.Int("n", 4, "total node number")
	f := flag.Int("f", 1, "byzantine node number")
	id := flag.Int("id", 0, "assign a unique number to different server")
	maxRound := flag.Int("mr", 1, "max round for each epoch")
	flag.Parse()

	prefixLogPath := "../" + *logPath
	logFile, err := os.OpenFile(prefixLogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("error opening file : %v", err)
	}

	defer logFile.Close()

	// Config logger
	logger := log.New(logFile, "logger: ", log.Ldate|log.Ltime|log.Lshortfile)
	logger.Print("Start server.")

	// Create and start network
	transport := libnet.NewNetworkTransport(logger, *port)
	// Create new consensus module
	cm := consensus.MakeConsensusModule(logger, transport, *n, *f, *id, *maxRound)
	cm.Run()
	// Start network
	transport.Start()
}
