package shared

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// oldStdout holds the original os.Stdout so that we can draw directly
// to the terminal even after hijacking os.Stdout into a pipe.
var oldStdout *os.File

// winsize is used for retrieving terminal dimensions via ioctl.
type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// GetTerminalSize returns the terminal’s number of rows and columns.
// If the ioctl fails, it defaults to 24 rows.
func GetTerminalSize() (int, int, error) {
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

// clearScreen sends the ANSI sequence to clear the entire terminal
// and move the cursor to row 1, column 1.
func clearScreen() {
	fmt.Fprint(oldStdout, "\033[2J\033[H")
}

// Redraw repaints the entire "buffer" of logs: first it displays the last (rows – 1) lines
// from logs, then draws a bold ">" prompt on the last row. Each entry in logs may
// already contain ANSI color codes.
func Redraw(logs []string) {
	rows, _, err := GetTerminalSize()
	if err != nil {
		rows = 24
	}
	clearScreen()

	// Reserve the top (rows – 1) lines for log history.
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

	// Draw a bold ">" prompt on the last row.
	fmt.Fprintf(oldStdout, "\033[%d;1H\033[1m> \033[0m", rows)
}

// HijackStdout redirects os.Stdout (and the default log output) into a pipe
// so that we can capture all fmt.Println or log.Println calls into outputCh.
// It also preserves the original stdout in oldStdout, so that Redraw/clearScreen
// can still write directly to the real terminal.
func HijackStdout(outputCh chan<- string) {
	r, w, _ := os.Pipe()
	oldStdout = os.Stdout
	os.Stdout = w
	log.SetOutput(w)

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
}
