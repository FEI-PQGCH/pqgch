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
	clients          = make(map[net.Conn]bool)
	muClients        sync.Mutex
)

type Message struct {
	ID       string `json:"id"`
	Sender   string `json:"sender"`
	Content  string `json:"content"`
	ClientID string `json:"client_id"` 
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

		// Add the new client to the clients map
		muClients.Lock()
		clients[conn] = true
		muClients.Unlock()
		fmt.Println("New client connected:", conn.RemoteAddr())

		// Handle connection in a new goroutine
		go handleConnection(conn, config)
	}
}

// handleConnection handles TCP connections and processes incoming messages.
func handleConnection(conn net.Conn, config shared.Config) {
	defer func() {
		// Remove the client from the clients map on disconnect.
		muClients.Lock()
		delete(clients, conn)
		muClients.Unlock()
		conn.Close()
		fmt.Println("Client disconnected:", conn.RemoteAddr())
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		msgData := scanner.Bytes()

		var msg Message
		err := json.Unmarshal(msgData, &msg)
		if err != nil {
			fmt.Println("Error unmarshaling message:", err)
			continue
		}

		// Check if the message has been received before.
		mu.Lock()
		if receivedMessages[msg.ID] {
			mu.Unlock()
			continue
		}
		// Mark the message as received.
		receivedMessages[msg.ID] = true
		mu.Unlock()

		// Process the message.
		fmt.Printf("Received message from %s: %s\n", msg.Sender, msg.Content)

		// Broadcast the message to all connected clients except the sender.
		broadcastMessage(msg, conn)

		// Forward the message to the left neighbor.
		forwardMessage(msg, config.LeftNeighbor)
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from client:", err)
	}
}


// broadcastMessage sends the given message to all connected clients.
func broadcastMessage(msg Message, senderConn net.Conn) {
	muClients.Lock()
	defer muClients.Unlock()

	for client := range clients {
		// Skip the client that originally sent the message.
		if client == senderConn {
			continue
		}

		msgData, err := json.Marshal(msg)
		if err != nil {
			fmt.Println("Error marshaling message:", err)
			continue
		}

		// Send message to the client.
		_, err = client.Write(append(msgData, '\n'))
		if err != nil {
			fmt.Println("Error sending message to client:", err)
			client.Close()
			delete(clients, client)
		}
	}
	fmt.Printf("Broadcasted message from %s: %s\n", msg.Sender, msg.Content)
}

// forwardMessage forwards the message to the left neighbor over TCP.
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
