package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"pqgch/cluster_protocol"
	"pqgch/shared"
	"strings"
	"syscall"
	"unsafe"
)

var (
	config    shared.UserConfig
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

	limit := rows - 1
	if limit < 0 {
		limit = 0
	}

	start := 0
	if len(logs) > limit {
		start = len(logs) - limit
	}
	for i := start; i < len(logs); i++ {
		fmt.Fprintln(oldStdout, logs[i])
	}

	fmt.Fprintf(oldStdout, "\033[%d;1H\033[1m> \033[0m", rows)
}

func main() {
	configFlag := flag.String("config", "", "path to configuration file")
	flag.Parse()
	if *configFlag == "" {
		log.Fatalln("[ERROR] Configuration file missing. Please provide it using the -config flag")
	}
	config = shared.GetUserConfig(*configFlag)

	// Redirect stdout into a pipe so we can capture internal logs and display them in TUI.
	r, w, _ := os.Pipe()
	oldStdout = os.Stdout
	os.Stdout = w
	log.SetOutput(w)

	// Channel to capture all output log lines.
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

	transport, _ := shared.NewTCPTransport(config.LeadAddr)
	loginMsg := shared.Message{
		ID:         shared.UniqueID(),
		SenderID:   config.Index,
		SenderName: config.GetName(),
		Type:       shared.LoginMsg,
		ClusterID:  config.ClusterConfig.Index,
	}
	transport.Send(loginMsg)

	// Create and initialize the cluster session.
	session := cluster_protocol.NewSession(transport, &config.ClusterConfig)
	session.Init()

	// Slice to store the history of all lines: internal logs and chat messages.
	var logs []string

	// Goroutine to read internal logs from outputCh and redraw the screen.
	go func() {
		for line := range outputCh {
			logs = append(logs, line)
			redraw(logs)
		}
	}()

	// Goroutine to receive chat messages from the server.
	go func() {
		for msg := range session.Received {
			colored := fmt.Sprintf("\033[32m%s: %s\033[0m", msg.SenderName, msg.Content)
			logs = append(logs, colored)
			redraw(logs)
		}
	}()

	// Goroutine to read user input (stdin).
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

	// Initial redraw: an empty screen with just the prompt.
	redraw(logs)

	for text := range input {
		colored := fmt.Sprintf("\033[32mYou: %s\033[0m", text)
		logs = append(logs, colored)
		session.SendText(text)
		redraw(logs)
	}
}
