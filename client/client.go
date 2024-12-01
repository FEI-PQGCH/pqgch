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
	mu      sync.Mutex
	config  shared.UserConfig
	session shared.Session
)

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()

	if *configFlag != "" {
		config = shared.GetUserConfig(*configFlag)
	} else {
		fmt.Println("please provide a configuration file using the -config flag")
		return
	}

	servAddr := config.LeadAddr
	conn, err := net.Dial("tcp", servAddr)
	if err != nil {
		fmt.Printf("error connecting to server %s: %v\n", servAddr, err)
		return
	}
	defer conn.Close()
	fmt.Printf("connected to server %s\n", servAddr)

	session.Xs = make([][32]byte, len(config.Names))
	loginMsg := shared.Message{
		MsgID:      uuid.New().String(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		MsgType:    shared.MsgLogin,
	}
	shared.SendMsg(conn, loginMsg)

	go receiveMsgs(conn)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("You: ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)

		switch text {
		case "":
			continue
		case "init":
			initProtocol(conn)
		default:
			broadcastMsg(conn, text)
		}
	}
}

func initProtocol(conn net.Conn) {
	fmt.Println("initiating the protocol")
	msg := shared.GetAkeInitAMsg(&session, config.ClusterConfig)
	fmt.Println("sending AKE A message")
	shared.SendMsg(conn, msg)
}

func broadcastMsg(conn net.Conn, text string) {
	if session.SharedSecret == [32]byte{} {
		fmt.Println("no shared secret, skipping")
		return
	}

	var cipherText, err = shared.EncryptAesGcm(text, &session)
	if err != nil {
		fmt.Println("error encrypting message")
		return
	}

	msg := shared.Message{
		MsgID:      uuid.New().String(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		Content:    cipherText,
		MsgType:    shared.MsgBroadcast,
	}
	shared.SendMsg(conn, msg)
}

func receiveMsgs(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var msg shared.Message
		err := json.Unmarshal(scanner.Bytes(), &msg)
		if err != nil {
			fmt.Println("error unmarshaling received message:", err)
			continue
		}

		handler := GetHandler(msg.MsgType)
		handler.HandleMessage(conn, msg)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("error reading from server:", err)
	}

	fmt.Println("disconnected from server")
}
