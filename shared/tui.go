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

func ComputeMaxOffset(logs []string) int {
	rows, _ := GetTerminalSize()
	limit := max(rows-1, 0)
	n := len(logs)
	if n <= limit {
		return 0
	}
	return n - limit
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
	limit := max(rows-1, 0)

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
