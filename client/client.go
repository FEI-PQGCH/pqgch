package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"pqgch/cluster_protocol"
	"pqgch/shared"
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

	transport, err := shared.NewTCPTransport(config.LeadAddr)
	if err != nil {
		fmt.Fprintf(os.Stdout, "[ERROR] Unable to connect to server: %v\n", err)
		os.Exit(1)
	}
	loginMsg := shared.Message{
		ID:         shared.UniqueID(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		Type:       shared.LoginMsg,
		ClusterID:  config.ClusterConfig.Index,
	}
	transport.Send(loginMsg)

	// Create and initialize cluster session.
	session := cluster_protocol.NewSession(transport, &config.ClusterConfig)
	session.Init()

	// Prepare scrollable log and input buffer.
	var logs []string
	scrollOffset := 0
	var inputBuffer []rune

	// Goroutine: capture internal stdout lines, append, redraw.
	go func() {
		for line := range outputCh {
			logs = append(logs, line)
			scrollOffset = shared.ComputeMaxOffset(logs)
			shared.Redraw(logs, scrollOffset, string(inputBuffer))
		}
	}()

	// Goroutine: capture incoming chat messages (green), append, redraw.
	go func() {
		for msg := range session.Received {
			colored := fmt.Sprintf("\033[32m%s: %s\033[0m", msg.SenderName, msg.Content)
			logs = append(logs, colored)
			scrollOffset = shared.ComputeMaxOffset(logs)
			shared.Redraw(logs, scrollOffset, string(inputBuffer))
		}
	}()

	// Start raw-mode input loop: ENTER→lineCh, arrows→scrollCh, chars/backspace→charCh.
	lineCh := make(chan string)
	scrollCh := make(chan int)
	charCh := make(chan rune)
	shared.StartInputLoop(lineCh, scrollCh, charCh)

	// Initial empty draw.
	shared.Redraw(logs, scrollOffset, "")

	// Main loop: handle ENTER, arrows, and character input.
	for {
		select {
		case _ = <-lineCh:
			text := string(inputBuffer)
			inputBuffer = inputBuffer[:0]
			colored := fmt.Sprintf("\033[32mYou: %s\033[0m", text)
			logs = append(logs, colored)
			session.SendText(text)
			scrollOffset = shared.ComputeMaxOffset(logs)
			shared.Redraw(logs, scrollOffset, "")

		case delta := <-scrollCh:
			newOffset := scrollOffset + delta
			maxOffset := shared.ComputeMaxOffset(logs)
			if newOffset < 0 {
				newOffset = 0
			}
			if newOffset > maxOffset {
				newOffset = maxOffset
			}
			scrollOffset = newOffset
			shared.Redraw(logs, scrollOffset, string(inputBuffer))

		case r := <-charCh:
			if r == 0 {
				if len(inputBuffer) > 0 {
					inputBuffer = inputBuffer[:len(inputBuffer)-1]
				}
			} else {
				inputBuffer = append(inputBuffer, r)
			}
			shared.Redraw(logs, scrollOffset, string(inputBuffer))
		}
	}
}
