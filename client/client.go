package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
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

	servAddr := config.LeadAddr
	conn, err := net.Dial("tcp", servAddr)
	if err != nil {
		fmt.Printf("[ERROR] Error connecting to server %s: %v\n", servAddr, err)
		panic("[ERROR] Error connecting to server")
	}
	defer conn.Close()
	fmt.Printf("[INFO] Connected to server %s\n", servAddr)

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

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("You: ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		devSession.SendText(text)
	}
}
