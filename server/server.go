package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"pqgch-client/gake"
	"pqgch-client/shared"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	name  string
	conn  net.Conn
	index int
}

var (
	receivedMessages   = make(map[string]bool)
	muReceivedMessages sync.Mutex
	clients            = make(map[Client]bool)
	muClients          sync.Mutex
	neighborConn       net.Conn
	muNeighborConn     sync.Mutex
	config             shared.ServConfig
	clusterSession     shared.Session
	mainSession        shared.Session
)

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()

	if *configFlag != "" {
		config = shared.GetServConfig(*configFlag)
	} else {
		fmt.Println("please provide a configuration file using the -config flag.")
		return
	}

	_, selfPort, err := net.SplitHostPort(config.GetCurrentServer())
	if err != nil {
		fmt.Println("error parsing self address from config:", err)
		return
	}
	port := selfPort

	address := fmt.Sprintf(":%s", port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("error starting TCP server:", err)
		return
	}
	defer listener.Close()
	fmt.Println("server listening on", address)
	clusterSession.Xs = make([][32]byte, len(config.Names))
	mainSession.Xs = make([][32]byte, len(config.ServAddrs))

	go connectNeighbor(config.GetRightNeighbor())

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("error accepting connection:", err)
			continue
		}

		clientLogin(conn)
	}
}

func clientLogin(conn net.Conn) {
	fmt.Println("new client connected:", conn.RemoteAddr())

	scanner := bufio.NewScanner(conn)
	scanner.Scan()
	msgData := scanner.Bytes()

	var msg shared.Message
	err := json.Unmarshal(msgData, &msg)
	if err != nil {
		fmt.Println("error unmarshaling message:", err)
		conn.Close()
	}

	if msg.Type != shared.LoginMsg {
		fmt.Println("client did not send login message")
		conn.Close()
	}

	client := Client{name: msg.SenderName, conn: conn, index: msg.SenderID}

	muClients.Lock()
	clients[client] = true
	muClients.Unlock()

	clusterClientCount := 0
	for c := range clients {
		if c.index != -1 {
			clusterClientCount++
		}
	}

	if clusterClientCount == len(config.Names)-1 {
		msg := shared.GetAkeAMsg(&clusterSession, &config.ClusterConfig)
		sendMsgToClient(msg)
	}

	go handleConnection(client)
}

func connectNeighbor(neighborAddress string) {
	for {
		muNeighborConn.Lock()
		if neighborConn == nil {
			fmt.Printf("connecting to right neighbor at %s\n", neighborAddress)
			conn, err := net.Dial("tcp", neighborAddress)
			if err != nil {
				fmt.Printf("error connecting to right neighbor: %v. Retrying...\n", err)
				muNeighborConn.Unlock()
				time.Sleep(2 * time.Second)
				continue
			}
			neighborConn = conn
			fmt.Printf("connected to right neighbor (%s)\n", neighborAddress)
			loginMsg := shared.Message{
				ID:         uuid.New().String(),
				SenderID:   -1,
				SenderName: "server",
				Type:       shared.LoginMsg,
			}

			err = shared.SendMsg(neighborConn, loginMsg)
			if err != nil {
				fmt.Printf("error sending login message to right neighbor: %v\n", err)
				neighborConn.Close()
				neighborConn = nil
			}

			aMsg := shared.GetAkeAMsg(&mainSession, &config)
			err = shared.SendMsg(neighborConn, aMsg)
			fmt.Println("CRYPTO: sending Leader AKE A message")

			if err != nil {
				fmt.Printf("error sending AkeInitA message to right neighbor: %v\n", err)
			}
			muNeighborConn.Unlock()
			break
		}
		muNeighborConn.Unlock()
		time.Sleep(5 * time.Second)
	}

	scanner := bufio.NewScanner(neighborConn)
	scanner.Scan()
	msgData := scanner.Bytes()

	var msg shared.Message
	err := json.Unmarshal(msgData, &msg)
	if err != nil {
		fmt.Println("error unmarshaling message:", err)
		return
	}

	if msg.Type == shared.LeaderAkeBMsg {
		fmt.Println("CRYPTO: received Leader AKE B message")
		akeSendB, _ := base64.StdEncoding.DecodeString(msg.Content)
		mainSession.KeyRight = gake.KexAkeSharedA(akeSendB, mainSession.TkRight, mainSession.EskaRight, config.GetDecodedSecretKey())

		fmt.Println("CRYPTO: established shared key with right neighbor")
		fmt.Printf("CRYPTO: KeyRight%d: %02x\n", config.Index, mainSession.KeyRight)

		ok, msg := shared.CheckLeftRightKeys(&mainSession, &config)

		if ok {
			fmt.Printf("CRYPTO: sending Xi\n")
			forwardMessage(msg)
		}
	} else {
		fmt.Println("error: expected Leader AKE B message")
	}
}

func handleConnection(client Client) {
	defer func() {
		muClients.Lock()
		delete(clients, client)
		muClients.Unlock()
		client.conn.Close()
		fmt.Println("client disconnected:", client.conn.RemoteAddr())
	}()

	scanner := bufio.NewScanner(client.conn)
	for scanner.Scan() {
		msgData := scanner.Bytes()

		var msg shared.Message
		err := json.Unmarshal(msgData, &msg)
		if err != nil {
			fmt.Println("error unmarshaling message:", err)
			continue
		}

		muReceivedMessages.Lock()
		if receivedMessages[msg.ID] {
			muReceivedMessages.Unlock()
			continue
		}
		receivedMessages[msg.ID] = true
		muReceivedMessages.Unlock()

		if client.index != -1 {
			msg.ClusterID = config.Index
		}

		fmt.Printf("RECEIVED: %s from %s \n", msg.MsgTypeName(), msg.SenderName)

		if msg.Type == shared.LeaderAkeAMsg {
			fmt.Println("CRYPTO: received Leader AKE A message")
			responseMsg := shared.GetAkeBMsg(&mainSession, msg, &config)
			shared.SendMsg(client.conn, responseMsg)
			fmt.Println("CRYPTO: sending Leader AKE B message")
			ok, msg := shared.CheckLeftRightKeys(&mainSession, &config)

			if ok {
				fmt.Printf("CRYPTO: sending Xi\n")
				forwardMessage(msg)
			}
			continue
		}

		if msg.Type == shared.LeaderXiMsg {
			fmt.Printf("CRYPTO: received Leader Xi (%d) message", msg.SenderID)

			if msg.SenderID != config.Index {
				xi, _ := base64.StdEncoding.DecodeString(msg.Content)
				var xiArr [32]byte
				copy(xiArr[:], xi)
				mainSession.Xs[msg.SenderID] = xiArr
				shared.CheckXs(&mainSession, &config)
				forwardMessage(msg)
			}
			continue
		}

		handler := GetHandler(msg)
		handler.HandleMessage(msg)
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("error reading from client:", err)
	}
}
