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

func getMasterkey(decodedContent []byte) {
	recoveredKey, err := shared.DecryptAndCheckHMAC(decodedContent, intraClusterKey[:])
	if err != nil {
		fmt.Println("error decrypting key message:", err)
		return
	}
	copy(masterKey[:], recoveredKey)
	fmt.Printf("[CRYPTO] Master key established: %02x\n", masterKey[:4])
}

func print(msg shared.Message) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("\r\033[K%s: %s\n", msg.SenderName, msg.Content)
	fmt.Print("You: ")
}

func keyHandler(msg shared.Message) {
	decodedContent, err := base64.StdEncoding.DecodeString(msg.Content)
	if err != nil {
		fmt.Printf("[ERROR] Failed to decrypt message\n")
		return
	}

	if intraClusterKey == [gake.SsLen]byte{} {
		fmt.Println("intraClusterKey not loaded yet")
		keyCiphertext = decodedContent
		return
	}
	getMasterkey(decodedContent)
}

func text(msg shared.Message) {
	plainText, err := shared.DecryptAesGcm(msg.Content, masterKey[:])
	if err != nil {
		fmt.Println("error decrypting message:", err)
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
		keyHandler(msg)
	default:
		text(msg)
	}
}
