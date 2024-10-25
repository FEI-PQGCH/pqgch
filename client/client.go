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
	// Parse command-line flags.
	configFlag := flag.String("config", "", "path to configuration file")
	nameFlag := flag.String("name", "", "name of the client")
	flag.Parse()

	// Load configuration.
	var config shared.Config
	if *configFlag != "" {
		config = shared.GetConfigFromPath(*configFlag)
	} else {
		fmt.Println("Please provide a configuration file using the -config flag.")
		return
	}

	// Check for a provided client name or prompt the user.
	if *nameFlag != "" {
		clientName = *nameFlag
	} else {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter your name: ")
		name, _ := reader.ReadString('\n')
		clientName = strings.TrimSpace(name)
	}

	// Generate a unique client ID.
	clientID = uuid.New().String()

	// Seed the random number generator.
	rand.Seed(time.Now().UnixNano())

	// Randomly select a server to connect to.
	serverConfig := config.Servers[rand.Intn(len(config.Servers))]
	address := fmt.Sprintf("%s:%d", serverConfig.Address, serverConfig.Port)

	// Connect to the server once.
	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Printf("Error connecting to server %s: %v\n", address, err)
		return
	}
	defer conn.Close()
	fmt.Printf("Connected to server %s\n", address)

	// Start a goroutine to listen for messages from the server.
	go receiveMessages(conn)

	// Read messages from stdin and send them to the server.
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter message: ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		// Create a message with a unique ID and the client ID.
		msg := Message{
			ID:       uuid.New().String(),
			Sender:   clientName,
			Content:  text,
			ClientID: clientID,
		}

		// Send the message.
		sendMessage(conn, msg)
	}
}

// receiveMessages reads messages from the server and prints them.
func receiveMessages(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var msg Message
		err := json.Unmarshal(scanner.Bytes(), &msg)
		if err != nil {
			fmt.Println("Error unmarshaling received message:", err)
			continue
		}

		// Ignore messages from the same client.
		if msg.ClientID == clientID {
			continue
		}

		// Print the message.
		printMessage(msg)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from server:", err)
	}

	fmt.Println("Disconnected from server")
}

// sendMessage sends a message to the server.
func sendMessage(conn net.Conn, msg Message) {
	msgData, err := json.Marshal(msg)
	if err != nil {
		fmt.Println("Error marshaling message:", err)
		return
	}

	// Append a newline character.
	msgData = append(msgData, '\n')

	// Send the message.
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
