package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"pqgch-client/shared"

	"github.com/google/uuid"
)

type Message struct {
	ID       string `json:"id"`
	Sender   string `json:"sender"`
	Content  string `json:"content"`
	ClientID string `json:"client_id"`
}

var (
	mu         sync.Mutex
	clientID   string
	clientName string
)

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	nameFlag := flag.String("name", "", "name of the client")
	flag.Parse()

	var config shared.Config
	if *configFlag != "" {
		config = shared.GetConfigFromPath(*configFlag)
	} else {
		fmt.Println("Please provide a configuration file using the -config flag.")
		return
	}

	if *nameFlag != "" {
		clientName = *nameFlag
	} else {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter your name: ")
		name, _ := reader.ReadString('\n')
		clientName = strings.TrimSpace(name)
	}

	clientID = uuid.New().String()

	rand.Seed(time.Now().UnixNano())

	serverConfig := config.Servers[rand.Intn(len(config.Servers))]
	address := fmt.Sprintf("%s:%d", serverConfig.Address, serverConfig.Port)

	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Printf("Error connecting to server %s: %v\n", address, err)
		return
	}
	defer conn.Close()
	fmt.Printf("Connected to server %s\n", address)

	go receiveMessages(conn)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("You: ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		msg := Message{
			ID:       uuid.New().String(),
			Sender:   clientName,
			Content:  text,
			ClientID: clientID,
		}
		sendMessage(conn, msg)
	}
}

func receiveMessages(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var msg Message
		err := json.Unmarshal(scanner.Bytes(), &msg)
		if err != nil {
			fmt.Println("Error unmarshaling received message:", err)
			continue
		}

		if msg.ClientID == clientID {
			continue
		}

		printMessage(msg)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from server:", err)
	}

	fmt.Println("Disconnected from server")
}

func sendMessage(conn net.Conn, msg Message) {
	msgData, err := json.Marshal(msg)
	if err != nil {
		fmt.Println("Error marshaling message:", err)
		return
	}
	msgData = append(msgData, '\n')
	_, err = conn.Write(msgData)
	if err != nil {
		fmt.Println("Error sending message:", err)
	}
}

func printMessage(msg Message) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("\r\033[K%s: %s\n", msg.Sender, msg.Content)
	fmt.Print("You: ")
}
