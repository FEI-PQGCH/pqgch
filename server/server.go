package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

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

	outputCh := make(chan string)
	shared.HijackStdout(outputCh)

	_, port, err := net.SplitHostPort(config.GetCurrentServer())
	if err != nil {
		fmt.Fprintln(os.Stdout, "[ERROR] Error parsing self address from config:", err)
		os.Exit(1)
	}
	address := fmt.Sprintf(":%s", port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Fprintln(os.Stdout, "[ERROR] Error starting TCP server:", err)
		os.Exit(1)
	}
	defer listener.Close()
	fmt.Fprintln(os.Stdout, "[ROUTE]: server listening on", address)

	// Create message tracker so we do not process the same message twice.
	tracker := shared.NewMessageTracker()

	// Initialize in-memory client registry.
	clients := newClients(&config.ClusterConfig)

	// Initialize leader transport and session.
	leaderTransport := newLeaderTransport()
	leaderSession := leader_protocol.NewSession(leaderTransport, &config)
	msgsLeader := make(chan shared.Message)
	go transportManager(leaderTransport, msgsLeader)

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

	// Goroutine for reading internal logs and redrawing TUI.
	var logs []string
	go func() {
		for line := range outputCh {
			logs = append(logs, line)
			shared.Redraw(logs)
		}
	}()

	// Goroutine for printing out received cluster messages in TUI.
	go func() {
		for msg := range clusterSession.Received {
			// Color received message green and append to logs.
			colored := fmt.Sprintf("\033[32m%s: %s\033[0m", msg.SenderName, msg.Content)
			logs = append(logs, colored)
			shared.Redraw(logs)
		}
	}()

	// Goroutine for handling operator input from stdin.
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

	// Initial TUI draw.
	shared.Redraw(logs)

	// Goroutine for accepting incoming TCP connections.
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Fprintln(os.Stdout, "[ERROR] Error accepting connection:", err)
				continue
			}
			go handleConnection(clients, conn, tracker, msgsCluster, msgsLeader)
		}
	}()

	// Main loop: wait for operator input to send messages via clusterSession.
	for text := range inputCh {
		// Color "You: …" green, append to logs, and send to cluster.
		colored := fmt.Sprintf("\033[32mYou: %s\033[0m", text)
		logs = append(logs, colored)
		clusterSession.SendText(text)
		shared.Redraw(logs)
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

	clients.makeOnline(msg.SenderID, conn)
	defer clients.makeOffline(msg.SenderID)

	// Send to the newly connected client every message from its queue.
	clients.sendQueued(msg.SenderID)

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
