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
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()

	var config shared.Config
	if *configFlag != "" {
		config = shared.GetConfigFromPath(*configFlag)
	} else {
		config = shared.GetConfig()
	}

	_, selfPort, err := net.SplitHostPort(config.SelfAddress)
	if err != nil {
		fmt.Println("Error parsing self address from config:", err)
		return
	}
	port := selfPort

	address := fmt.Sprintf(":%s", port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("Error starting TCP server:", err)
		return
	}
	defer listener.Close()
	fmt.Println("Server listening on", address)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		muClients.Lock()
		clients[conn] = true
		muClients.Unlock()
		fmt.Println("New client connected:", conn.RemoteAddr())

		go handleConnection(conn, config)
	}
}

func handleConnection(conn net.Conn, config shared.Config) {
	defer func() {
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

		mu.Lock()
		if receivedMessages[msg.ID] {
			mu.Unlock()
			continue
		}
		receivedMessages[msg.ID] = true
		mu.Unlock()

		fmt.Printf("Received message from %s: %s\n", msg.Sender, msg.Content)

		broadcastMessage(msg, conn)

		forwardMessage(msg, config.LeftNeighbor)
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from client:", err)
	}
}


func broadcastMessage(msg Message, senderConn net.Conn) {
	muClients.Lock()
	defer muClients.Unlock()

	for client := range clients {
		if client == senderConn {
			continue
		}

		msgData, err := json.Marshal(msg)
		if err != nil {
			fmt.Println("Error marshaling message:", err)
			continue
		}

		_, err = client.Write(append(msgData, '\n'))
		if err != nil {
			fmt.Println("Error sending message to client:", err)
			client.Close()
			delete(clients, client)
		}
	}
	fmt.Printf("Broadcasted message from %s: %s\n", msg.Sender, msg.Content)
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

	msgData = append(msgData, '\n')

	_, err = conn.Write(msgData)
	if err != nil {
		fmt.Println("Error forwarding message to left neighbor:", err)
	} else {
		fmt.Printf("Message forwarded to left neighbor (%s)\n", neighborAddress)
	}
}
