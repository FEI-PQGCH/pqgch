package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"

	"pqgch-client/shared"

	"github.com/google/uuid"
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
        fmt.Println("Please provide a configuration file using the -config flag.")
        return
    }

    // Seed the random number generator
    rand.Seed(time.Now().UnixNano())

    reader := bufio.NewReader(os.Stdin)
    for {
        fmt.Print("Enter message: ")
        text, _ := reader.ReadString('\n')

        // Create a message with a unique ID
        msg := Message{
            ID:      uuid.New().String(),
            Sender:  "Client",
            Content: text,
        }

        // Randomly select a server to send the message to
        serverConfig := config.Servers[rand.Intn(len(config.Servers))]
        address := fmt.Sprintf("%s:%d", serverConfig.Address, serverConfig.Port)

        // Connect to the server
        conn, err := net.Dial("tcp", address)
        if err != nil {
            fmt.Printf("Error connecting to server %s: %v\n", address, err)
            continue
        }
        fmt.Printf("Connected to server %s\n", address)

        // Send the message
        // Existing code to marshal the message
				msgData, err := json.Marshal(msg)
				if err != nil {
						fmt.Println("Error marshaling message:", err)
						conn.Close()
						continue
				}

				// Append a newline character
				msgData = append(msgData, '\n')

				// Send the message
				_, err = conn.Write(msgData)
				if err != nil {
						fmt.Println("Error sending message:", err)
						conn.Close()
						continue
				}

						}
}
