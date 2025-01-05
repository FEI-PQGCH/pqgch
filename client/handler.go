package main

import (
	"fmt"
	"net"
	"pqgch-client/shared"
)

func akeAHandler(conn net.Conn, msg shared.Message) {
	akeB, xi := shared.HandleAkeA(msg, &config, &session)
	akeB.Send(conn)
	if !xi.IsEmpty() {
		xi.Send(conn)
	}
}

func akeBHandler(conn net.Conn, msg shared.Message) {
	xi := shared.HandleAkeB(msg, &config, &session)
	if !xi.IsEmpty() {
		xi.Send(conn)
	}
}

func xiHandler(msg shared.Message) {
	shared.HandleXi(msg, &config, &session)
}

func printMessage(msg shared.Message) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("\r\033[K%s: %s\n", msg.SenderName, msg.Content)
	fmt.Print("You: ")
}

func defaultHandler(msg shared.Message) {
	var plainText, err = shared.DecryptAesGcm(msg.Content, &session)
	if err != nil {
		fmt.Println("error decrypting message")
		return
	}

	msg.Content = plainText
	printMessage(msg)
}

func handleMessage(conn net.Conn, msg shared.Message) {
	switch msg.Type {
	case shared.AkeAMsg:
		akeAHandler(conn, msg)
	case shared.AkeBMsg:
		akeBHandler(conn, msg)
	case shared.XiMsg:
		xiHandler(msg)
	default:
		defaultHandler(msg)
	}
}
