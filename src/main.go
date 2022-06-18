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
	cm := consensus.MakeConsensusModule(logger, transport)
	cm.Run()
	// Start network
	transport.Start()
}
