package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"pqgch/cluster_protocol"
	"pqgch/leader_protocol"
	"pqgch/shared"
	"strings"
)

var (
	config shared.ServConfig
)

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()

	// Load config.
	if *configFlag != "" {
		config = shared.GetServConfig(*configFlag)
	} else {
		log.Fatalln("[ERROR] Configuration file missing. Please provide it using the -config flag.")
	}

	// Start listening at configured port.
	_, port, err := net.SplitHostPort(config.GetCurrentServer())
	if err != nil {
		log.Fatalln("[ERROR] Error parsing self address from config:", err)
	}
	address := fmt.Sprintf(":%s", port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalln("[ERROR] Error starting TCP server:", err)
	}
	defer listener.Close()
	fmt.Println("[ROUTE]: server listening on", address)

	// Create message tracker so we do not process the same message twice.
	tracker := shared.NewMessageTracker()

	clients := newClients(&config.ClusterConfig)

	// Initialize transports and sessions.
	leaderTransport := newLeaderTransport()
	leaderSession := leader_protocol.NewSession(leaderTransport, &config)
	msgsLeader := make(chan shared.Message)
	go transportManager(leaderTransport, msgsLeader)

	clusterTransport := newClusterTransport(clients)
	clusterSession := cluster_protocol.NewLeaderSession(clusterTransport, &config.ClusterConfig, leaderSession.GetKeyRef())
	msgsCluster := make(chan shared.Message)
	go transportManager(clusterTransport, msgsCluster)

	leaderSession.Init()
	clusterSession.Init()

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			text, _ := reader.ReadString('\n')
			text = strings.TrimSpace(text)
			if text != "" {
				clusterSession.SendText(text)
			}
		}
	}()

	go func() {
		for {
			msg := <-clusterSession.Received
			fmt.Printf("\r\033[K%s: %s\n", msg.SenderName, msg.Content)
		}
	}()

	// Handle incoming connections.
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("[ERROR] Error accepting connection:", err)
			continue
		}
		go handleConnection(clients, conn, tracker, msgsCluster, msgsLeader)
	}
}

func handleConnection(
	clients *Clients,
	conn net.Conn,
	tracker *shared.MessageTracker,
	clusterChan chan shared.Message,
	leaderChan chan shared.Message,
) {
	reader := shared.NewMessageReader(conn)
	// Verify that the client sent some message.
	if !reader.HasMessage() {
		fmt.Println("[ERROR] Client did not send any message")
		conn.Close()
		return
	}

	// Check whether message is a Login message.
	// If it is not, we received a message from some other cluster leader.
	// We handle the Text message explicitly by broadcasting it to our cluster.
	// We handle other messages through Leader Transport since they shoould
	// be a part of the protocol between leaders.
	msg := reader.GetMessage()
	if msg.Type != shared.LoginMsg {
		if !tracker.AddMessage(msg.ID) {
			return
		}
		fmt.Printf("[ROUTE] Received %s message from Leader\n", msg.TypeName())
		if msg.Type == shared.TextMsg {
			clusterChan <- msg
			clients.broadcast(msg)
			return
		}

		leaderChan <- msg
		conn.Close()
		return
	}

	// If the message sent was a Login message (which comes from clients), we save the client and his connection in memory.
	fmt.Printf("[INFO] New client (%s, %s) joined\n", msg.SenderName, conn.RemoteAddr())

	clients.makeOnline(msg.SenderID, conn)
	defer func() {
		clients.makeOffline(msg.SenderID)
	}()

	// Send to the newly connected client every message from his queue.
	clients.sendQueued(msg.SenderID)

	// Handle messages from this client in an infinite loop.
	for reader.HasMessage() {
		msg := reader.GetMessage()
		fmt.Printf("[ROUTE] Received %s from %s \n", msg.TypeName(), msg.SenderName)

		if !tracker.AddMessage(msg.ID) {
			continue
		}

		msg.ClusterID = config.Index

		switch msg.Type {
		case shared.AkeAMsg, shared.AkeBMsg:
			if msg.ReceiverID == config.ClusterConfig.Index {
				clusterChan <- msg
			} else {
				clients.send(msg)
			}
		case shared.XiRiCommitmentMsg:
			clusterChan <- msg
			clients.broadcast(msg)
		case shared.TextMsg:
			clusterChan <- msg
			clients.broadcast(msg)
			broadcastToLeaders(msg)
		case shared.LeaderAkeAMsg, shared.LeaderAkeBMsg, shared.LeaderXiRiCommitmentMsg:
			leaderChan <- msg
		}
	}
}
