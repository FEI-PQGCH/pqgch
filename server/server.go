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

	outputCh := make(chan string)
	shared.HijackStdout(outputCh)

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

	var logs []string
	scrollOffset := 0
	var inputBuffer []rune

	computeMaxOffset := func() int {
		rows, _ := shared.GetTerminalSize()
		limit := rows - 1
		if limit < 0 {
			limit = 0
		}
		n := len(logs)
		if n <= limit {
			return 0
		}
		return n - limit
	}

	go func() {
		for line := range outputCh {
			logs = append(logs, line)
			scrollOffset = computeMaxOffset()
			shared.Redraw(logs, scrollOffset, string(inputBuffer))
		}
	}()

	// Goroutine: capture incoming cluster messages, append, redraw.
	go func() {
		for msg := range clusterSession.Received {
			colored := fmt.Sprintf("\033[32m%s: %s\033[0m", msg.SenderName, msg.Content)
			logs = append(logs, colored)
			scrollOffset = computeMaxOffset()
			shared.Redraw(logs, scrollOffset, string(inputBuffer))
		}
	}()

	lineCh := make(chan string)
	scrollCh := make(chan int)
	charCh := make(chan rune)
	shared.StartInputLoop(lineCh, scrollCh, charCh)

	// Initial empty draw.
	shared.Redraw(logs, scrollOffset, "")

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

	// Main loop: handle ENTER, arrows, and character input.
	for {
		select {
		case <-lineCh:
			text := string(inputBuffer)
			inputBuffer = inputBuffer[:0]
			colored := fmt.Sprintf("\033[32mYou: %s\033[0m", text)
			logs = append(logs, colored)
			clusterSession.SendText(text)
			scrollOffset = computeMaxOffset()
			shared.Redraw(logs, scrollOffset, "")

		case delta := <-scrollCh:
			newOffset := scrollOffset + delta
			maxOffset := computeMaxOffset()
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
