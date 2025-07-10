package util

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"unsafe"
)

type winsize struct {
	rows    uint16
	cols    uint16
	xpixels uint16
	ypixels uint16
}

func getTerminalSize() (rows, cols int) {
	ws := &winsize{}
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if errno != 0 {
		panic(errno)
	}
	return int(ws.rows), int(ws.cols)
}

const (
	esc                = "\033["
	alternateScreenOn  = esc + "?1049h"
	alternateScreenOff = esc + "?1049l"
	clearLineCode      = esc + "2K"
	clearScreen        = esc + "2J\033[H"
	cursorShow         = esc + "?25h"
	cursorHide         = esc + "?25l"
)

func enterAlternateScreen() {
	fmt.Print(alternateScreenOn)
}

func exitAlternateScreen() {
	fmt.Print(alternateScreenOff)
}

func moveToLine(sb *strings.Builder, i int) {
	fmt.Fprintf(sb, "%s%d;%dH", esc, i, 1)
}

func clearLine(sb *strings.Builder) {
	fmt.Fprint(sb, clearLineCode)
}

func print(sb *strings.Builder, m string) {
	fmt.Fprint(sb, m)
}

func printLineAt(sb *strings.Builder, m string, i int) {
	moveToLine(sb, i)
	clearLine(sb)
	print(sb, m)
}

var oldTermios syscall.Termios

func enableRawMode() {
	fd := int(os.Stdin.Fd())
	var newt syscall.Termios

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

func wrapLines(lines []Log, width int) []Log {
	var wrapped []Log
	for _, line := range lines {
		current := ""
		for _, r := range line.Text {
			if len(current) >= width {
				wrapped = append(wrapped, Log{Text: current, Color: line.Color})
				current = ""
			}
			current += string(r)
		}
		if current != "" {
			wrapped = append(wrapped, Log{Text: current, Color: line.Color})
		}
	}
	return wrapped
}

func wrapInput(input string, width int) []Log {
	wrapped := []Log{}
	line := "> "
	for _, r := range input {
		if len(line) >= width {
			wrapped = append(wrapped, Log{Text: line, Color: ColorWhite})
			line = "> "
		}
		line += string(r)
	}
	wrapped = append(wrapped, Log{Text: line, Color: ColorWhite})
	return wrapped
}

type Color string

const (
	ColorReset Color = "\033[0m"
	ColorRed   Color = "\033[31m"
	ColorGreen Color = "\033[32m"
	ColorBlue  Color = "\033[34m"
	ColorCyan  Color = "\033[36m"
	ColorWhite Color = "\033[37m"
)

func colorize(msg string, color Color) string {
	return string(color) + msg + string(ColorReset)
}

type Log struct {
	Text  string
	Color Color
}

var lineChan = make(chan Log, 100)

func PrintLine(msg string) {
	PrintLineColored(msg, ColorWhite)
}

func PrintLineColored(msg string, color Color) {
	lineChan <- Log{Text: msg, Color: color}
}

var errChan = make(chan Log, 1)

func FatalError(msg string) {
	errChan <- Log{Text: msg, Color: ColorRed}
}

func exit() {
	exitAlternateScreen()
	disableRawMode()
	os.Exit(0)
}

func StartTUI(onLine func(string)) {
	// Initialization
	enterAlternateScreen()
	defer exitAlternateScreen()
	fmt.Print(clearScreen)

	enableRawMode()
	defer disableRawMode()

	// CTRL-C handler
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		exit()
	}()

	// Input loop
	inputReader := bufio.NewReader(os.Stdin)
	keyboardCh := make(chan rune, 256)
	go func() {
		for {
			r, _, err := inputReader.ReadRune()
			if err != nil {
				close(keyboardCh)
				return
			}
			// ESC
			if r == '\x1b' {
				if inputReader.Buffered() >= 2 {
					r1, _, _ := inputReader.ReadRune()
					r2, _, _ := inputReader.ReadRune()
					if r1 == '[' {
						switch r2 {
						case 'A':
							keyboardCh <- '↑'
							continue
						case 'B':
							keyboardCh <- '↓'
							continue
						}
					}
				} else {
					exit()
				}
				continue
			}
			keyboardCh <- r
		}
	}()

	// Render loop
	input := []rune{}
	inputHeight := 0
	lastInputHeight := 0
	logs := []Log{}
	scrollOffset := 0
	lastFrame := []string{}
	failed := false

	for {
		rows, cols := getTerminalSize()

		if len(lastFrame) != max(rows-inputHeight, 0) {
			lastFrame = make([]string, max(rows-inputHeight, 0))
		}

		wrappedInput := wrapInput(string(input), cols)
		wrappedMessages := wrapLines(logs, cols)

		lastInputHeight = inputHeight
		inputHeight = len(wrappedInput)

		viewSpace := max(rows-inputHeight, 0)

		start := max(len(wrappedMessages)-viewSpace, 0)
		end := len(wrappedMessages)

		start = max(start-scrollOffset, 0)
		end -= scrollOffset

		if len(wrappedMessages) >= viewSpace && end < viewSpace {
			end = viewSpace
		}

		view := wrappedMessages[start:end]

		var sb strings.Builder
		// Clear old input space
		if inputHeight < lastInputHeight {
			from := rows - lastInputHeight + 1
			for i := range lastInputHeight - inputHeight {
				moveToLine(&sb, from+i)
				clearLine(&sb)
			}
		}
		if len(lastFrame) != len(view) {
			lastFrame = make([]string, len(view))
		}

		// Draw messages
		for i, line := range view {
			if i >= len(lastFrame) || lastFrame[i] != line.Text {
				printLineAt(&sb, colorize(line.Text, line.Color), i+1)
				lastFrame[i] = view[i].Text
			}
		}

		// Draw input
		from := rows - inputHeight + 1
		for i, line := range wrappedInput {
			printLineAt(&sb, colorize(line.Text, line.Color), from+i)
		}

		// Render
		os.Stdout.Write([]byte(sb.String()))

		// Get next input and update state accordingly
		fmt.Print(cursorShow)
		select {
		case key := <-keyboardCh:
			if failed {
				exit()
			}

			switch key {
			case '\r', '\n':
				if len(input) == 0 {
					continue
				}
				logs = append(logs, Log{Text: "You: " + string(input), Color: ColorGreen})
				onLine(string(input))
				input = []rune{}
				continue
			case 127:
				if len(input) > 0 {
					input = input[:len(input)-1]
				}
				continue
			case '↑':
				scrollOffset++
				maxScroll := max(len(wrappedMessages)-viewSpace, 0)
				scrollOffset = min(scrollOffset, maxScroll)
				continue
			case '↓':
				scrollOffset--
				scrollOffset = max(scrollOffset, 0)
				continue
			default:
				input = append(input, key)
				continue
			}
		case line := <-lineChan:
			if !failed {
				logs = append(logs, line)
			}
		case err := <-errChan:
			logs = append(logs, err)
			logs = append(logs, Log{Text: "Press any key to exit", Color: ColorRed})
			failed = true
		}
		fmt.Print(cursorHide)
	}
}
