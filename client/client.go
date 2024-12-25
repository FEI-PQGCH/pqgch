package main

import (
	"bufio"
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

func initialize() net.Conn {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()

	if *configFlag != "" {
		config = shared.GetUserConfig(*configFlag)
	} else {
		fmt.Println("please provide a configuration file using the -config flag")
		panic("no configuration file provided")
	}

	servAddr := config.LeadAddr
	conn, err := net.Dial("tcp", servAddr)
	if err != nil {
		fmt.Printf("error connecting to server %s: %v\n", servAddr, err)
		panic("error connecting to server")
	}

	fmt.Printf("connected to server %s\n", servAddr)

	session = shared.MakeSession(&config.ClusterConfig)
	return conn
}

func main() {
	conn := initialize()
	defer conn.Close()

	loginMsg := shared.Message{
		ID:         uuid.New().String(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		Type:       shared.LoginMsg,
		ClusterID:  -1,
	}
	loginMsg.Send(conn)

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
	fmt.Println("CRYPTO: initiating the protocol")
	msg := shared.GetAkeAMsg(&session, &config.ClusterConfig)
	fmt.Println("CRYPTO: sending AKE A message")
	msg.Send(conn)
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
		ID:         uuid.New().String(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		Content:    cipherText,
		Type:       shared.BroadcastMsg,
	}
	msg.Send(conn)
}

func receiveMsgs(conn net.Conn) {
	msgReader := shared.NewMessageReader(conn)

	for msgReader.HasMessage() {
		msg := msgReader.GetMessage()
		fmt.Printf("RECEIVED: %s from %s\n", msg.MsgTypeName(), msg.SenderName)
		handler := GetHandler(msg.Type)
		handler.HandleMessage(conn, msg)
	}

	fmt.Println("disconnected from server")
}
