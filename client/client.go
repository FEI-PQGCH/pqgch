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
	mu            sync.Mutex
	config        shared.UserConfig
	session       shared.Session
	key           [32]byte
	keyCiphertext []byte
	debugMode     bool
)

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	debugFlag := flag.Bool("debug", true, "enable debug messages")
	flag.Parse()

	shared.DebugMode = *debugFlag

	if *configFlag != "" {
		config = shared.GetUserConfig(*configFlag)
	} else {
		fmt.Printf("[ERROR] Configuration file missing. Please provide it using the -config flag\n")
		panic("no configuration file provided")
	}

	servAddr := config.LeadAddr
	conn, err := net.Dial("tcp", servAddr)
	if err != nil {
		fmt.Printf("error connecting to server %s: %v\n", servAddr, err)
		panic("error connecting to server")
	}
	defer conn.Close()
	fmt.Printf("[INFO] Connected to server %s\n", servAddr)

	session = shared.MakeSession(&config.ClusterConfig)
	session.OnSharedKey = func() {
		if keyCiphertext == nil {
			fmt.Println("[CRYPTO] No key ciphertext, skipping")
			return
		}
		getMainKey(keyCiphertext)
	}

	loginMsg := shared.Message{
		ID:         uuid.New().String(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		Type:       shared.LoginMsg,
		ClusterID:  -1,
	}
	loginMsg.Send(conn)

	go receiver(conn)

	initProtocol(conn)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("You: ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		send(conn, text)
	}
}

func initProtocol(conn net.Conn) {
	fmt.Printf("[CRYPTO] Initiating protocol (AKE A)\n")
	msg := shared.GetAkeAMsg(&session, &config.ClusterConfig)
	fmt.Printf("[CRYPTO] Sending AKE A message\n")
	msg.Send(conn)
}

func send(conn net.Conn, text string) {
	if session.SharedSecret == [64]byte{} {
		fmt.Printf("[ERROR] Shared secret not established – message skipped\n")
		return
	}

	var cipherText, err = shared.EncryptAesGcm(text, key[:])
	if err != nil {
		fmt.Printf("[ERROR] Encryption failed: %v\n", err)
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

func receiver(conn net.Conn) {
	msgReader := shared.NewMessageReader(conn)

	for msgReader.HasMessage() {
		msg := msgReader.GetMessage()
		if debugMode {
			fmt.Printf("[DEBUG] Received %s from %s\n", msg.TypeName(), msg.SenderName)
		}
		handleMessage(conn, msg)
	}

	fmt.Println("[INFO] Disconnected from server")
}
