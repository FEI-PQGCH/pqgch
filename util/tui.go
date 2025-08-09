package util

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

const (
	esc           = "\033["
	clearLineCode = esc + "2K"
	cursorShow    = esc + "?25h"
	cursorHide    = esc + "?25l"
)

type winsize struct {
	rows    uint16
	cols    uint16
	xpixels uint16
	ypixels uint16
}

func getTerminalWidth() (cols int) {
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
	return int(ws.cols)
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

	fmt.Print(cursorHide)
}

func disableRawMode() {
	fd := int(os.Stdin.Fd())
	syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&oldTermios)),
	)
	fmt.Print(cursorShow)
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
	PrintLineColored(msg, ColorReset)
}

func PrintLineColored(msg string, color Color) {
	lineChan <- Log{Text: msg, Color: color}
}

func ExitWithMsg(msg string) {
	clearLine()
	fmt.Println(colorize(msg, ColorRed))
	exit()
}

func exit() {
	disableRawMode()
	os.Exit(0)
}

func printPrompt(input string) {
	terminalWidth := getTerminalWidth()
	promptLen := len(input) + 2
	start := max(promptLen-terminalWidth, 0)
	clearLine()
	fmt.Print("> ")

	if len(input) > 0 {
		fmt.Print(string(input[start:]))
	} else {
		fmt.Print(colorize("Enter your message here", ColorWhite))
	}
}

func clearLine() {
	fmt.Print("\r" + clearLineCode)
}

func StartTUI(onLine func(string)) {
	// Initialization
	enableRawMode()
	defer disableRawMode()

	// CTRL-C handler
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		exit()
	}()

	printPrompt("")
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
					// Keyboard arrows
					if r1 == '[' {
						switch r2 {
						case 'A':
							continue
						case 'B':
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

	input := []rune{}
	for {
		select {
		case key := <-keyboardCh:
			switch key {
			case '\r', '\n':
				if len(input) == 0 {
					printPrompt("")
					continue
				}
				clearLine()
				log := "You: " + string(input)
				fmt.Println(colorize(log, ColorGreen))
				onLine(string(input))
				input = []rune{}
				printPrompt("")
				continue
			case 127:
				if len(input) > 0 {
					input = input[:len(input)-1]
				}
				printPrompt(string(input))
				continue
			default:
				input = append(input, key)
				printPrompt(string(input))
				continue
			}
		case line := <-lineChan:
			clearLine()
			fmt.Println(colorize(line.Text, line.Color))
			printPrompt(string(input))
		}
	}
}
