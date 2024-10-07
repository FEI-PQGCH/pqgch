package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

var (
	clusters      = map[string]map[net.Conn]bool{"A": {}, "B": {}, "C": {}}
	messages      = make(chan string)
	clientCluster = make(map[net.Conn]string)
	mutex         = sync.Mutex{}
)

func handleConnection(conn net.Conn) {
	defer conn.Close()

	clientAddr := conn.RemoteAddr().String()
	fmt.Fprintf(conn, "Welcome! Choose a cluster (A, B, C): ")
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return
	}
	chosenCluster := strings.ToUpper(scanner.Text())
	if chosenCluster != "A" && chosenCluster != "B" && chosenCluster != "C" {
		fmt.Fprintln(conn, "Invalid cluster choice. Disconnecting...")
		return
	}

	// Register the client in the chosen cluster
	mutex.Lock()
	clusters[chosenCluster][conn] = true
	clientCluster[conn] = chosenCluster
	mutex.Unlock()

	fmt.Printf("%s connected to cluster %s\n", clientAddr, chosenCluster)
	fmt.Fprintf(conn, "You have joined cluster %s. Type '/quit' to disconnect.\n", chosenCluster)

	// Read client messages
	for scanner.Scan() {
		text := scanner.Text()
		if text == "/quit" {
			break
		}
		msg := fmt.Sprintf("[%s] %s: %s", chosenCluster, clientAddr, text)
		messages <- msg
	}

	// Client disconnected
	fmt.Printf("%s disconnected from cluster %s\n", clientAddr, chosenCluster)
	mutex.Lock()
	delete(clusters[chosenCluster], conn)
	delete(clientCluster, conn)
	mutex.Unlock()
}

func broadcastMessages() {
	for msg := range messages {
		parts := strings.SplitN(msg, " ", 2)
		if len(parts) < 2 {
			continue
		}
		clusterTag := strings.Trim(parts[0], "[]")
		message := parts[1]

		mutex.Lock()
		for conn := range clusters[clusterTag] {
			fmt.Fprintln(conn, message)
		}
		mutex.Unlock()
	}
}

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Error starting server:", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("Server started on port 8080 with clusters A, B, C")

	go broadcastMessages()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		go handleConnection(conn)
	}
}
