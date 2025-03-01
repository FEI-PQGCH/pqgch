package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"pqgch-client/gake"
	"pqgch-client/shared"
	"strings"
	"sync"

	"github.com/google/uuid"
)

var (
	mu              sync.Mutex
	config          shared.UserConfig
	session         shared.Session
	masterKey       [gake.SsLen]byte
	intraClusterKey [gake.SsLen]byte
	keyCiphertext   []byte
	debugMode       bool
	useExternal     bool
)

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	debugFlag := flag.Bool("debug", true, "enable debug messages")
	useExternalFlag := flag.Bool("useExternal", false, "use external key from file for key derivation")
	flag.Parse()
	useExternal = *useExternalFlag

	shared.DebugMode = *debugFlag

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

	session = shared.MakeSession(&config.ClusterConfig)
	session.OnSharedKey = func() {
		if keyCiphertext == nil {
			fmt.Println("[CRYPTO] No key ciphertext, skipping")
			return
		}
		getMasterKey(keyCiphertext)
	}

	loginMsg := shared.Message{
		ID:         uuid.New().String(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		Type:       shared.LoginMsg,
		ClusterID:  config.ClusterConfig.Index,
	}
	loginMsg.Send(conn)

	go receiver(conn)
	if useExternal {
		keyFilePath := config.ClusterConfig.ClusterKeyFile
		loadedKey, err := shared.LoadClusterKey(keyFilePath)
		if err != nil {
			fmt.Printf("[ERROR] Error loading cluster key from file %s: %v\n", keyFilePath, err)
			panic(err)
		}
		intraClusterKey = loadedKey
		fmt.Printf("[CRYPTO] external cluster key loaded from file %s\n", keyFilePath)
		fmt.Printf("[CRYPTO] intra-cluster key initialized: %02x\n", intraClusterKey[:4])
		if keyCiphertext != nil {
			session.OnSharedKey()
		}
	} else {
		initProtocol(conn)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("You: ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		send(conn, text)
	}
}

func initProtocol(conn net.Conn) {
	fmt.Println("[CRYPTO] Initiating the protocol")
	msg := shared.GetAkeAMsg(&session, &config.ClusterConfig)
	fmt.Println("[CRYPTO] Sending AKE A message")
	msg.Send(conn)
}

func send(conn net.Conn, text string) {
	if masterKey == [gake.SsLen]byte{} {
		fmt.Println("[CRYPTO] No master key available, skipping")
		return
	}
	cipherText, err := shared.EncryptAesGcm(text, masterKey[:])
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
		ClusterID:  config.ClusterConfig.Index,
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
