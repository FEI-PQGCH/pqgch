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
	"syscall"
	"unsafe"
)

var (
	config    shared.ServConfig
	oldStdout *os.File
)

// winsize is the structure used to obtain terminal dimensions via ioctl.
type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// getTerminalSize returns the number of rows and columns of the current terminal.
// If it fails, it defaults to 24 rows.
func getTerminalSize() (int, int, error) {
	ws := &winsize{}
	ret, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(oldStdout.Fd()),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if int(ret) == -1 {
		return 0, 0, errno
	}
	return int(ws.Row), int(ws.Col), nil
}

// clearScreen clears the entire terminal and moves the cursor to the top-left corner.
func clearScreen() {
	fmt.Fprint(oldStdout, "\033[2J\033[H")
}

// redraw repaints the entire "buffer" of logs: first it displays the last (rows-1) lines from logs,
// then draws a bold ">" prompt on the last row.
func redraw(logs []string) {
	rows, _, err := getTerminalSize()
	if err != nil {
		rows = 24
	}
	clearScreen()

	// Reserve the top (rows-1) lines for log history.
	limit := rows - 1
	if limit < 0 {
		limit = 0
	}

	// If we have more entries than the limit, only display the last 'limit' entries.
	start := 0
	if len(logs) > limit {
		start = len(logs) - limit
	}
	for i := start; i < len(logs); i++ {
		fmt.Fprintln(oldStdout, logs[i])
	}

	// Draw a bold ">" prompt on the last row.
	fmt.Fprintf(oldStdout, "\033[%d;1H\033[1m> \033[0m", rows)
}

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()

	// Load config.
	if *configFlag != "" {
		config = shared.GetServConfig(*configFlag)
	} else {
		log.Fatalln("[ERROR] Configuration file missing. Please provide it using the -config flag.")
	}

	// Redirect standard output into a pipe so we can capture internal logs and display them in TUI.
	r, w, _ := os.Pipe()
	oldStdout = os.Stdout
	os.Stdout = w
	log.SetOutput(w)

	// Channel for capturing all output log lines.
	outputCh := make(chan string)
	go func() {
		reader := bufio.NewReader(r)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				close(outputCh)
				return
			}
			outputCh <- strings.TrimSuffix(line, "\n")
		}
	}()

	// Start listening at configured port.
	_, port, err := net.SplitHostPort(config.GetCurrentServer())
	if err != nil {
		fmt.Fprintln(oldStdout, "[ERROR] Error parsing self address from config:", err)
		os.Exit(1)
	}
	address := fmt.Sprintf(":%s", port)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Fprintln(oldStdout, "[ERROR] Error starting TCP server:", err)
		os.Exit(1)
	}
	defer listener.Close()
	fmt.Fprintln(oldStdout, "[ROUTE]: server listening on", address)

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

	// Goroutine for reading internal logs and redrawing TUI.
	var logs []string
	go func() {
		for line := range outputCh {
			logs = append(logs, line)
			redraw(logs)
		}
	}()

	// Goroutine for printing out received cluster messages in TUI.
	go func() {
		for msg := range clusterSession.Received {
			// Color received message green and append to logs.
			colored := fmt.Sprintf("\033[32m%s: %s\033[0m", msg.SenderName, msg.Content)
			logs = append(logs, colored)
			redraw(logs)
		}
	}()

	// Goroutine for handling user input from stdin.
	input := make(chan string)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			text, _ := reader.ReadString('\n')
			text = strings.TrimSpace(text)
			if text != "" {
				input <- text
			}
		}
	}()

	// Initial TUI draw.
	redraw(logs)

	// Goroutine for accepting incoming TCP connections.
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Fprintln(oldStdout, "[ERROR] Error accepting connection:", err)
				continue
			}
			go handleConnection(clients, conn, tracker, msgsCluster, msgsLeader)
		}
	}()

	// Main loop: wait for operator input to send messages via clusterSession.
	for text := range input {
		// Color "You: …" green, append to logs, and send to cluster.
		colored := fmt.Sprintf("\033[32mYou: %s\033[0m", text)
		logs = append(logs, colored)
		clusterSession.SendText(text)
		redraw(logs)
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
		fmt.Fprintln(oldStdout, "[ERROR] Client did not send any message")
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
		// Log receipt of a leader protocol message.
		coloredLog := fmt.Sprintf("\033[33m[ROUTE] Received %s message from Leader\033[0m", msg.TypeName())
		fmt.Fprintln(oldStdout, coloredLog)
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
	coloredLog := fmt.Sprintf("\033[33m[INFO] New client (%s, %s) joined\033[0m", msg.SenderName, conn.RemoteAddr())
	fmt.Fprintln(oldStdout, coloredLog)
	clients.makeOnline(msg.SenderID, conn)
	defer clients.makeOffline(msg.SenderID)

	// Send to the newly connected client every message from his queue.
	clients.sendQueued(msg.SenderID)

	// Handle messages from this client in an infinite loop.
	for reader.HasMessage() {
		msg := reader.GetMessage()
		fmt.Fprintf(oldStdout, "\033[33m[ROUTE] Received %s from %s\033[0m\n", msg.TypeName(), msg.SenderName)

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
