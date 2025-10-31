package util

import (
	"bufio"
	"fmt"
	"os"

	"golang.org/x/term"
)

const (
	esc           = "\033["
	clearLineCode = esc + "2K"
	cursorShow    = esc + "?25h"
	cursorHide    = esc + "?25l"
)

type Color string

const (
	ColorReset  Color = "\033[0m"
	ColorRed    Color = "\033[31m"
	ColorGreen  Color = "\033[32m"
	ColorYellow Color = "\033[33m"
	ColorBlue   Color = "\033[34m"
	ColorPurple Color = "\033[35m"
	ColorCyan   Color = "\033[36m"
	ColorWhite  Color = "\033[37m"
)

func colorize(msg string, color Color) string {
	return string(color) + msg + string(ColorReset)
}

var logChan = make(chan string, 100)
var msgChan = make(chan string, 100)

func LogInfo(msg string) {
	header := colorize("[INFO] ", ColorYellow)
	logChan <- header + msg
}

func LogRoute(msg string) {
	header := colorize("[ROUTE] ", ColorPurple)
	logChan <- header + msg
}

func LogRouteWithNames(form string, item string, from string, to string) {
	header := colorize("[ROUTE] ", ColorPurple)
	body := colorize(form, ColorBlue) + " " + item + " " + from + " " + colorize(to, ColorBlue)
	logChan <- header + body
}

func LogError(msg string) {
	header := colorize("[ERROR] ", ColorRed)
	logChan <- header + msg
}

func LogCrypto(msg string) {
	header := colorize("[CRYPTO] ", ColorCyan)
	logChan <- header + msg
}

func PrintLine(msg string) {
	PrintLineColored(msg, ColorReset)
}

func PrintLineColored(msg string, color Color) {
	msgChan <- colorize(msg, color)
}

func ExitWithMsg(msg string) {
	clearLine()
	header := colorize("[ERROR] ", ColorRed)
	content := colorize(msg, ColorRed)
	fmt.Println(header + content)
	exit()
}

func exit() {
	term.Restore(int(os.Stdin.Fd()), oldState)
	fmt.Print("\r")
	fmt.Print(cursorShow)
	os.Exit(0)
}

func printPrompt(input string) {
	terminalWidth, _, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		terminalWidth = 80
	}

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

var oldState *term.State

func StartTUI(onLine func(string)) {
	var err error
	oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)
	fmt.Print(cursorHide)
	defer fmt.Print(cursorShow)

	printPrompt("")

	reader := bufio.NewReader(os.Stdin)
	chars := make(chan rune, 256)
	go func() {
		for {
			r, _, err := reader.ReadRune()
			if err != nil {
				close(chars)
				return
			}
			// CTRL-C
			if r == '\x03' {
				exit()
			}
			// ESC
			if r == '\x1b' {
				n := reader.Buffered()
				if n > 0 {
					reader.Discard(n)
				} else {
					exit()
				}
				continue
			}
			chars <- r
		}
	}()

	// Input loop
	input := []rune{}
	for {
		select {
		case log := <-logChan:
			clearLine()
			fmt.Fprintln(os.Stderr, log)
			printPrompt(string(input))
		case msg := <-msgChan:
			clearLine()
			fmt.Fprintln(os.Stdout, msg)
			printPrompt(string(input))
		case r := <-chars:
			switch r {
			case '\r', '\n':
				if len(input) == 0 {
					printPrompt("")
					continue
				}
				clearLine()
				log := "You: " + string(input)
				fmt.Fprintln(os.Stdout, colorize(log, ColorGreen))

				onLine(string(input))

				input = []rune{}
				printPrompt("")
			case 127:
				if len(input) > 0 {
					input = input[:len(input)-1]
				}
				printPrompt(string(input))
			default:
				input = append(input, r)
				printPrompt(string(input))
			}
		}
	}
}
