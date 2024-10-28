package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"sync"
	"time"

	"pqgch-client/shared"
)

var (
	receivedMessages = make(map[string]bool)
	mu               sync.Mutex
	clients          = make(map[net.Conn]bool)
	muClients        sync.Mutex
	neighborConn     net.Conn
	neighborConnMu   sync.Mutex
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

	go maintainNeighborConnection(config.LeftNeighbor)

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

		go handleConnection(conn)
	}
}

func maintainNeighborConnection(neighborAddress string) {
	for {
		neighborConnMu.Lock()
		if neighborConn == nil {
			fmt.Printf("Connecting to left neighbor at %s\n", neighborAddress)
			conn, err := net.Dial("tcp", neighborAddress)
			if err != nil {
				fmt.Printf("Error connecting to left neighbor: %v. Retrying...\n", err)
				neighborConnMu.Unlock()
				time.Sleep(2 * time.Second)
				continue
			}
			neighborConn = conn
			fmt.Printf("Connected to left neighbor (%s)\n", neighborAddress)
		}
		neighborConnMu.Unlock()
		time.Sleep(5 * time.Second)
	}
}

func handleConnection(conn net.Conn) {
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
		forwardMessage(msg)
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

func forwardMessage(msg Message) {
	neighborConnMu.Lock()
	defer neighborConnMu.Unlock()

	if neighborConn == nil {
		fmt.Println("No connection to left neighbor; message not forwarded.")
		return
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		fmt.Println("Error marshaling message:", err)
		return
	}

	msgData = append(msgData, '\n')

	_, err = neighborConn.Write(msgData)
	if err != nil {
		fmt.Printf("Error forwarding message to left neighbor: %v\n", err)
		neighborConn.Close()
		neighborConn = nil 
	} else {
		fmt.Printf("Message forwarded to left neighbor\n")
	}
}
