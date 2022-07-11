package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"time"

	"github.com/zhazhalaila/PipelineBFT/src/consensus"
	"github.com/zhazhalaila/PipelineBFT/src/libnet"
)

func readAddress(path string, n int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		n--
		if n < 0 {
			break
		}
		lines = append(lines, scanner.Text())
	}

	if scanner.Err() != nil {
		return nil, err
	}

	return lines, nil
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
	logger := log.New(logFile, "logger: ", log.Ldate|log.Ltime|log.Lshortfile)
	logger.Print("Start server.")

	// Create and start network
	transport := libnet.NewNetworkTransport(logger, *port)

	// Create new consensus module
	cm := consensus.MakeConsensusModule(logger, transport, *n, *f, *id, *maxRound)
	cm.Run()

	// Read address
	addresses, err := readAddress("../address.txt", *n)
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

	// Start network
	transport.Start()
}
