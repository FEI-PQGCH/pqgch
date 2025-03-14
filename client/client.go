package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"pqgch-client/shared"
	"strings"

	"github.com/google/uuid"
)

var (
	config shared.UserConfig
)

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()

	if *configFlag != "" {
		config = shared.GetUserConfig(*configFlag)
	} else {
		fmt.Printf("[ERROR] Configuration file missing. Please provide it using the -config flag\n")
		panic("no configuration file provided")
	}
	config = shared.GetUserConfig(*configFlag)

	transport, _ := shared.NewTCPTransport(config.LeadAddr)
	loginMsg := shared.Message{
		ID:         uuid.New().String(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		Type:       shared.LoginMsg,
		ClusterID:  config.ClusterConfig.Index,
	}
	transport.Send(loginMsg)
	devSession := shared.NewDevSession(transport, &config.ClusterConfig)
	devSession.Init()

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

	for {
		fmt.Print("You: ")
		select {
		case msg := <-devSession.Received:
			fmt.Printf("\r\033[K%s: %s\n", msg.SenderName, msg.Content)

		case text := <-input:
			devSession.SendText(text)
		}
	}
}
