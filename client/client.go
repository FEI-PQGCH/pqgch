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

var config shared.UserConfig

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()
	if *configFlag == "" {
		log.Fatalln("[ERROR] Configuration file missing. Please provide it using the -config flag")
	}
	config = shared.GetUserConfig(*configFlag)

	outputCh := make(chan string)
	shared.HijackStdout(outputCh)

	transport, _ := shared.NewTCPTransport(config.LeadAddr)
	loginMsg := shared.Message{
		ID:         shared.UniqueID(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		Type:       shared.LoginMsg,
		ClusterID:  config.ClusterConfig.Index,
	}
	transport.Send(loginMsg)

	// Create and initialize the cluster session.
	session := cluster_protocol.NewSession(transport, &config.ClusterConfig)
	session.Init()

	// Our in‐memory "scrollback" of all lines we want to display.
	var logs []string

	// 1) Goroutine: capture anything that got printed/logged internally, then Redraw.
	go func() {
		for line := range outputCh {
			logs = append(logs, line)
			shared.Redraw(logs)
		}
	}()

	// 2) Goroutine: capture incoming chat messages, color them green, append, and Redraw.
	go func() {
		for msg := range session.Received {
			colored := fmt.Sprintf("\033[32m%s: %s\033[0m", msg.SenderName, msg.Content)
			logs = append(logs, colored)
			shared.Redraw(logs)
		}
	}()

	// 3) Goroutine: read user input from stdin and push onto a channel.
	inputCh := make(chan string)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			text, _ := reader.ReadString('\n')
			text = strings.TrimSpace(text)
			if text != "" {
				inputCh <- text
			}
		}
	}()

	// 4) Initial draw (empty).
	shared.Redraw(logs)

	for text := range inputCh {
		// Color "You: …" green, append to logs, send to server, Redraw.
		colored := fmt.Sprintf("\033[32mYou: %s\033[0m", text)
		logs = append(logs, colored)
		session.SendText(text)
		shared.Redraw(logs)
	}
}
