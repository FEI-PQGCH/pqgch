package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"pqgch/cluster_protocol"
	"pqgch/shared"
	"strings"
)

var (
	config shared.UserConfig
)

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()

	// Load config.
	if *configFlag != "" {
		config = shared.GetUserConfig(*configFlag)
	} else {
		log.Fatalln("[ERROR] Configuration file missing. Please provide it using the -config flag")
	}
	config = shared.GetUserConfig(*configFlag)

	// Create TCP transport (connection to server).
	transport, _ := shared.NewTCPTransport(config.LeadAddr)
	loginMsg := shared.Message{
		ID:         shared.UniqueID(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		Type:       shared.LoginMsg,
		ClusterID:  config.ClusterConfig.Index,
	}
	// Login to the server, so it recognizes us as a client.
	transport.Send(loginMsg)

	// Create and initialize the cluster session.
	session := cluster_protocol.NewSession(transport, &config.ClusterConfig)
	session.Init()

	// Goroutine for reading user input.
	input := make(chan string)
	reader := bufio.NewReader(os.Stdin)
	go func() {
		for {
			text, _ := reader.ReadString('\n')
			text = strings.TrimSpace(text)
			if text != "" {
				input <- text
			}
		}
	}()

	// Infinite loop for printing out received messages and sending the input ones.
	for {
		fmt.Print("You: ")
		select {
		case msg := <-session.Received:
			fmt.Printf("\r\033[K%s: %s\n", msg.SenderName, msg.Content)

		case text := <-input:
			session.SendText(text)
		}
	}
}
