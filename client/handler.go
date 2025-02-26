package main

import (
	"encoding/base64"
	"fmt"
	"net"
	"pqgch-client/gake"
	"pqgch-client/shared"
)

func akeA(conn net.Conn, msg shared.Message) {
	akeB, xi := shared.HandleAkeA(msg, &config, &session)
	akeB.Send(conn)
	if !xi.IsEmpty() {
		xi.Send(conn)
	}
}

func akeB(conn net.Conn, msg shared.Message) {
	xi := shared.HandleAkeB(msg, &config, &session)
	if !xi.IsEmpty() {
		xi.Send(conn)
	}
}

func xi(msg shared.Message) {
	shared.HandleXi(msg, &config, &session)
}

func sharedKey(msg shared.Message) {
	decodedContent, _ := base64.StdEncoding.DecodeString(msg.Content)

	if session.SharedSecret == [gake.SsLen]byte{} {
		keyCiphertext = decodedContent
		return
	}
	getMainKey(decodedContent)
}

func getMainKey(decodedContent []byte) {
	decryptedKey, err := shared.DecryptAndCheckHMAC(decodedContent, &session)
	if err != nil {
		fmt.Println("error: decrypting key")
		return
	}
	copy(key[:], decryptedKey)

	fmt.Printf("CRYPTO: main shared key established: %02x\n", key[:4])
}

func print(msg shared.Message) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("\r\033[K%s: %s\n", msg.SenderName, msg.Content)
	fmt.Print("You: ")
}

func text(msg shared.Message) {
	var plainText, err = shared.DecryptAesGcm(msg.Content, key[:])
	if err != nil {
		fmt.Println("error decrypting message")
		return
	}

	msg.Content = plainText
	print(msg)
}

func handleMessage(conn net.Conn, msg shared.Message) {
	switch msg.Type {
	case shared.AkeAMsg:
		akeA(conn, msg)
	case shared.AkeBMsg:
		akeB(conn, msg)
	case shared.XiMsg:
		xi(msg)
	case shared.KeyMsg:
		sharedKey(msg)
	default:
		text(msg)
	}
}
