package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/zhazhalaila/PipelineBFT/src/consensus"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
	readaddress "github.com/zhazhalaila/PipelineBFT/src/readAddress"
)

func PrintMemUsage(logger *log.Logger) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	logger.Printf("Alloc = %v MiB.\n", bToMb(m.Alloc))
	logger.Printf("Sys = %v MiB\n", bToMb(m.Sys))
	logger.Printf("NumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

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
	logger := log.New(logFile, "logger: ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
	logger.Print("Start server.")

	// Create and start network
	transport := libnet.NewNetworkTransport(logger, *port)

	// Create new consensus module
	cm := consensus.MakeConsensusModule(logger, transport, *n, *f, *id, *maxRound)
	cm.Run()

	// Read address
	addresses, err := readaddress.ReadAddress("../address.txt", *n)
	if err != nil {
		log.Fatal(err)
	}

	// Wait for server start.
	go func() {
		time.Sleep(30 * time.Second)
		err := transport.ConnectAll(addresses)
		if err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		for {
			time.Sleep(10 * time.Second)
			logger.Printf("Runtime Goroutine = [%d].\n", runtime.NumGoroutine())
			PrintMemUsage(logger)
		}
	}()

	// Start network
	transport.Start()
}
