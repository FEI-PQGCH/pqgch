package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"pqgch/cluster_protocol"
	"pqgch/leader_protocol"
	"pqgch/shared"
)

var config shared.ServConfig

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()
	if *configFlag == "" {
		log.Fatalln("[ERROR] Configuration file missing. Please provide it using the -config flag.")
	}
	config = shared.GetServConfig(*configFlag)

	tui := shared.NewTUI()
	tui.HijackStdout()

	_, port, err := net.SplitHostPort(config.GetCurrentServer())
	if err != nil {
		fmt.Fprintf(os.Stdout, "[ERROR] Error parsing self address from config: %v\n", err)
		os.Exit(1)
	}
	address := fmt.Sprintf(":%s", port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Fprintf(os.Stdout, "[ERROR] Error starting TCP server: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()
	fmt.Fprintf(os.Stdout, "[ROUTE]: server listening on %s\n", address)

	// Create message tracker and in-memory client registry.
	tracker := shared.NewMessageTracker()
	clients := newClients(&config.ClusterConfig)

	// Initialize leader transport/session.
	leaderTransport := newLeaderTransport()
	leaderSession := leader_protocol.NewSession(leaderTransport, &config)
	msgsLeader := make(chan shared.Message)
	go transportManager(leaderTransport, msgsLeader)

	// Initialize cluster transport/session.
	clusterTransport := newClusterTransport(clients)
	clusterSession := cluster_protocol.NewLeaderSession(
		clusterTransport,
		&config.ClusterConfig,
		leaderSession.GetKeyRef(),
	)
	msgsCluster := make(chan shared.Message)
	go transportManager(clusterTransport, msgsCluster)

	leaderSession.Init()
	clusterSession.Init()

	// Wire incoming cluster messages into the TUI.
	tui.AttachMessages(clusterSession.Received)

	// Accept connections in a goroutine.
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Fprintf(os.Stdout, "[ERROR] Error accepting connection: %v\n", err)
				continue
			}
			go handleConnection(clients, conn, tracker, msgsCluster, msgsLeader)
		}
	}()

	// Run the TUI event loop; on ENTER send text via cluster session.
	tui.Run(func(line string) {
		clusterSession.SendText(line)
	})
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
		fmt.Fprintln(os.Stdout, "[ERROR] Client did not send any message")
		conn.Close()
		return
	}

	// Check whether message is a Login message.
	// If it is not, we received a message from some other cluster leader.
	// We handle the Text message explicitly by broadcasting it to our cluster.
	// We handle other messages through Leader Transport since they should
	// be a part of the protocol between leaders.
	msg := reader.GetMessage()
	if msg.Type != shared.LoginMsg {
		if !tracker.AddMessage(msg.ID) {
			return
		}
		// Log receipt of a leader protocol message.
		fmt.Fprintf(os.Stdout, "[ROUTE] Received %s message from Leader\n", msg.TypeName())
		if msg.Type == shared.TextMsg {
			clusterChan <- msg
			clients.broadcast(msg)
			return
		}
		leaderChan <- msg
		conn.Close()
		return
	}

	// Handle client login.
	fmt.Fprintf(os.Stdout, "[INFO] New client (%s, %s) joined\n", msg.SenderName, conn.RemoteAddr())
	clientID := msg.SenderID

	clients.makeOnline(clientID, conn)
	defer clients.makeOffline(clientID)

	// Send to the newly connected client every message from its queue.
	clients.sendQueued(clientID)

	// Handle messages from this client in an infinite loop.
	for reader.HasMessage() {
		msg := reader.GetMessage()
		fmt.Fprintf(os.Stdout, "[ROUTE] Received %s from %s\n", msg.TypeName(), msg.SenderName)

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
