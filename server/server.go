package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"sync"

	"pqgch-client/shared"
)

var (
    receivedMessages = make(map[string]bool)
    mu               sync.Mutex
)

type Message struct {
    ID      string `json:"id"`
    Sender  string `json:"sender"`
    Content string `json:"content"`
}

func main() {
    // Parse command-line flags
    configFlag := flag.String("config", "", "path to configuration file")
    flag.Parse()

    // Load configuration
    var config shared.Config
    if *configFlag != "" {
        config = shared.GetConfigFromPath(*configFlag)
    } else {
        config = shared.GetConfig()
    }

    // Get the port from command-line arguments or from config
    _, selfPort, err := net.SplitHostPort(config.SelfAddress)
    if err != nil {
        fmt.Println("Error parsing self address from config:", err)
        return
    }
    port := selfPort

    // Server address and port
    address := fmt.Sprintf(":%s", port)

    // Start listening for incoming connections
    listener, err := net.Listen("tcp", address)
    if err != nil {
        fmt.Println("Error starting TCP server:", err)
        return
    }
    defer listener.Close()
    fmt.Println("Server listening on", address)

    for {
        // Accept incoming connection
        conn, err := listener.Accept()
        if err != nil {
            fmt.Println("Error accepting connection:", err)
            continue
        }

        // Handle connection in a new goroutine
        go handleConnection(conn, config)
    }
}

func handleConnection(conn net.Conn, config shared.Config) {
    defer conn.Close()
    fmt.Println("New client connected:", conn.RemoteAddr())

    scanner := bufio.NewScanner(conn)
    for scanner.Scan() {
        msgData := scanner.Bytes()

        var msg Message
        err := json.Unmarshal(msgData, &msg)
        if err != nil {
            fmt.Println("Error unmarshaling message:", err)
            continue
        }

        // Check if message has been received before
        mu.Lock()
        if receivedMessages[msg.ID] {
            mu.Unlock()
            continue
        }
        // Mark message as received
        receivedMessages[msg.ID] = true
        mu.Unlock()

        // Process the message
        fmt.Printf("Received message from %s: %s\n", msg.Sender, msg.Content)

        // Forward the message to the left neighbor
        forwardMessage(msg, config.LeftNeighbor)
    }
    if err := scanner.Err(); err != nil {
        fmt.Println("Error reading from client:", err)
    }
    fmt.Println("Client disconnected:", conn.RemoteAddr())
}

func forwardMessage(msg Message, neighborAddress string) {
	conn, err := net.Dial("tcp", neighborAddress)
	if err != nil {
			fmt.Printf("Error connecting to left neighbor (%s): %v\n", neighborAddress, err)
			return
	}
	defer conn.Close()

	msgData, err := json.Marshal(msg)
	if err != nil {
			fmt.Println("Error marshaling message:", err)
			return
	}

	// Append a newline character
	msgData = append(msgData, '\n')

	_, err = conn.Write(msgData)
	if err != nil {
			fmt.Println("Error forwarding message to left neighbor:", err)
	} else {
			fmt.Printf("Message forwarded to left neighbor (%s)\n", neighborAddress)
	}
}

