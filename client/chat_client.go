package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main() {
	// Connect to server
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Handle incoming messages in a goroutine
	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	// Send the chosen cluster and user messages
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Enter your messages (type '/quit' to exit):")

	for scanner.Scan() {
		text := scanner.Text()
		if text == "/quit" {
			fmt.Println("Exiting...")
			break
		}
		fmt.Fprintln(conn, text)
	}
}
