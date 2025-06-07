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
func GetTerminalSize() (int, int) {
	ws := &winsize{}
	r1, _, _ := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(oldStdout.Fd()),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if int(r1) == -1 {
		return 24, 80
	}
	return int(ws.Row), int(ws.Col)
}

// clearScreen sends the ANSI sequence to clear the entire terminal
// and move the cursor to row 1, column 1.
func clearScreen() {
	fmt.Fprint(oldStdout, "\033[2J\033[H")
}

// Redraw repaints the entire "buffer" of logs: first it displays the last (rows – 1) lines
// from logs, then draws a bold ">" prompt on the last row. Each entry in logs may
// already contain ANSI color codes.
func Redraw(logs []string, scrollOffset int, inputBuffer string) {
	rows, _ := GetTerminalSize()
	limit := rows - 1
	if limit < 0 {
		limit = 0
	}

	clearScreen()

	n := len(logs)
	if n <= limit {
		for _, line := range logs {
			fmt.Fprintln(oldStdout, line)
		}
	} else {
		// Clamp scrollOffset in [0, n-limit].
		if scrollOffset < 0 {
			scrollOffset = 0
		}
		maxOffset := n - limit
		if scrollOffset > maxOffset {
			scrollOffset = maxOffset
		}
		start := scrollOffset
		end := scrollOffset + limit
		for i := start; i < end; i++ {
			fmt.Fprintln(oldStdout, logs[i])
		}
	}

	// Bold prompt + inputBuffer on last line.
	fmt.Fprintf(oldStdout, "\033[%d;1H\033[1m> %s\033[0m", rows, inputBuffer)
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

type termios struct {
	Iflag, Oflag, Cflag, Lflag uint32
	Cc                         [20]byte
	Ispeed, Ospeed             uint32
}

var oldTermios termios

func enableRawMode() {
	fd := int(os.Stdin.Fd())
	var newt termios

	syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&oldTermios)),
	)

	newt = oldTermios
	newt.Lflag &^= (syscall.ICANON | syscall.ECHO)
	newt.Cc[syscall.VMIN] = 1
	newt.Cc[syscall.VTIME] = 0

	syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&newt)),
	)
}

func disableRawMode() {
	fd := int(os.Stdin.Fd())
	syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&oldTermios)),
	)
}

// StartInputLoop reads raw keystrokes from os.Stdin and demultiplexes them:
//   - ENTER → send "" on lineCh.
//   - UP arrow → send -1 on scrollCh.
//   - DOWN arrow → send +1 on scrollCh.
//   - Backspace (127) → send rune(0) on charCh.
//   - Other runes → send that rune on charCh.
func StartInputLoop(lineCh chan<- string, scrollCh chan<- int, charCh chan<- rune) {
	enableRawMode()
	go func() {
		defer disableRawMode()

		reader := bufio.NewReader(os.Stdin)
		for {
			r, _, err := reader.ReadRune()
			if err != nil {
				return
			}
			switch r {
			case '\r', '\n':
				lineCh <- ""
			case 127:
				charCh <- 0
			case '\x1b':
				_, _, err2 := reader.ReadRune()
				if err2 != nil {
					continue
				}
				third, _, err3 := reader.ReadRune()
				if err3 != nil {
					continue
				}
				if third == 'A' {
					scrollCh <- -1
				} else if third == 'B' {
					scrollCh <- +1
				}
			default:
				charCh <- r
			}
		}
	}()
}

// TUI encapsulates a scrollable log pane and input prompt.
type TUI struct {
	logs         []string
	inputBuf     []rune
	scrollOffset int

	outCh    chan string
	msgCh    <-chan Message
	lineCh   chan string
	scrollCh chan int
	charCh   chan rune
}

// NewTUI creates a fresh TUI instance.
func NewTUI() *TUI {
	return &TUI{
		logs:         make([]string, 0, 100),
		inputBuf:     make([]rune, 0, 256),
		scrollOffset: 0,
		outCh:        make(chan string),
		lineCh:       make(chan string),
		scrollCh:     make(chan int),
		charCh:       make(chan rune),
	}
}

// HijackStdout hooks fmt/log output into the TUI's outCh.
func (t *TUI) HijackStdout() {
	HijackStdout(t.outCh)
}

// AttachMessages connects an incoming chat channel to the TUI.
func (t *TUI) AttachMessages(ch <-chan Message) {
	t.msgCh = ch
}

// Run starts the event loop, redrawing on any update.
func (t *TUI) Run(onLine func(string)) {
	go func() {
		for line := range t.outCh {
			t.appendLine(line)
		}
	}()
	go func() {
		for msg := range t.msgCh {
			col := fmt.Sprintf("\033[32m%s: %s\033[0m", msg.SenderName, msg.Content)
			t.appendLine(col)
		}
	}()

	StartInputLoop(t.lineCh, t.scrollCh, t.charCh)
	t.redraw()

	for {
		select {
		case <-t.lineCh:
			text := string(t.inputBuf)
			t.inputBuf = t.inputBuf[:0]
			onLine(text)
			t.appendLine(fmt.Sprintf("\033[32mYou: %s\033[0m", text))

		case delta := <-t.scrollCh:
			t.scrollOffset += delta
			if t.scrollOffset < 0 {
				t.scrollOffset = 0
			}
			if mo := t.maxOffset(); t.scrollOffset > mo {
				t.scrollOffset = mo
			}
			t.redraw()

		case r := <-t.charCh:
			if r == 0 {
				if len(t.inputBuf) > 0 {
					t.inputBuf = t.inputBuf[:len(t.inputBuf)-1]
				}
			} else {
				t.inputBuf = append(t.inputBuf, r)
			}
			t.redraw()
		}
	}
}

// appendLine adds a new log line, pins view to bottom, and redraws.
func (t *TUI) appendLine(line string) {
	t.logs = append(t.logs, line)
	t.scrollOffset = t.maxOffset()
	t.redraw()
}

// maxOffset computes how far the log can scroll up.
func (t *TUI) maxOffset() int {
	rows, _ := GetTerminalSize()
	limit := rows - 1
	if limit < 0 {
		limit = 0
	}
	n := len(t.logs)
	if n <= limit {
		return 0
	}
	return n - limit
}

// redraw calls the shared Redraw helper with current state.
func (t *TUI) redraw() {
	Redraw(t.logs, t.scrollOffset, string(t.inputBuf))
}
