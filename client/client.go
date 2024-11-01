package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"pqgch-client/shared"
	"strings"
	"sync"

	"github.com/google/uuid"
)

var (
	mu         sync.Mutex
	clientName string
)

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()

	var config shared.UserConfig
	if *configFlag != "" {
		config = shared.GetUserConfig(*configFlag)
	} else {
		fmt.Println("Please provide a configuration file using the -config flag.")
		return
	}

	servAddr := config.LeadAddr
	clientName = config.Names[config.Index]

	conn, err := net.Dial("tcp", servAddr)
	if err != nil {
		fmt.Printf("Error connecting to server %s: %v\n", servAddr, err)
		return
	}
	defer conn.Close()
	fmt.Printf("Connected to server %s\n", servAddr)

	loginMsg := shared.Message{
		MsgID:      uuid.New().String(),
		SenderID:   config.Index,
		SenderName: clientName,
		MsgType:    shared.MsgLogin,
	}

	sendMessage(conn, loginMsg)

	go receiveMessages(conn)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("You: ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		msg := shared.Message{
			MsgID:      uuid.New().String(),
			SenderID:   config.Index,
			SenderName: clientName,
			Content:    text,
			MsgType:    shared.MsgBroadcast,

			// For specific client:
			//MsgType:    shared.MsgIntra,
			//ReceiverID: 1,
		}
		sendMessage(conn, msg)
	}
}

func receiveMessages(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var msg shared.Message
		err := json.Unmarshal(scanner.Bytes(), &msg)
		if err != nil {
			fmt.Println("Error unmarshaling received message:", err)
			continue
		}

		printMessage(msg)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from server:", err)
	}

	fmt.Println("Disconnected from server")
}

func sendMessage(conn net.Conn, msg shared.Message) {
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

func printMessage(msg shared.Message) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("\r\033[K%s: %s\n", msg.SenderName, msg.Content)
	fmt.Print("You: ")
}
