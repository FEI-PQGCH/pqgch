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

func print(msg shared.Message) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("\r\033[K%s: %s\n", msg.SenderName, msg.Content)
	fmt.Print("You: ")
}

func keyHandler(msg shared.Message) {
	decodedContent, err := base64.StdEncoding.DecodeString(msg.Content)
	if err != nil {
		fmt.Printf("[ERROR] Failed to decode key message: %v\n", err)
		return
	}
	var wrappingKey [64]byte
	var zero [gake.SsLen]byte
	if useExternal {
		if intraClusterKey == zero {
			fmt.Println("[CRYPTO] External key not loaded yet; storing key message")
			keyCiphertext = decodedContent
			return
		}
		copy(wrappingKey[:], intraClusterKey[:])
		fmt.Printf("[DEBUG] Using external key (intraClusterKey): %x\n", intraClusterKey[:])
	} else {
		if session.SharedSecret == zero {
			fmt.Println("[CRYPTO] GAKE handshake not complete; storing key message")
			keyCiphertext = decodedContent
			return
		}
		copy(wrappingKey[:], session.SharedSecret[:])
	}
	recoveredKey, err := shared.DecryptAndCheckHMAC(decodedContent, wrappingKey[:])
	if err != nil {
		fmt.Println("[ERROR] Failed decrypting key message:", err)
		return
	}
	copy(masterKey[:], recoveredKey)
}

func getMasterKey(decodedContent []byte) {
	fmt.Println("[CRYPTO] inside getMasterkey")

	var keyForDecryption [64]byte
	if useExternal {
		copy(keyForDecryption[:], intraClusterKey[:])
	} else {
		copy(keyForDecryption[:], session.SharedSecret[:])
	}
	recoveredKey, err := shared.DecryptAndCheckHMAC(decodedContent, keyForDecryption[:])

	if err != nil {
		fmt.Println("[ERROR] Failed decrypting key message:", err)
		return
	}
	copy(masterKey[:], recoveredKey)
	fmt.Printf("[CRYPTO] Master key established: %02x\n", masterKey[:4])
}

func text(msg shared.Message) {
	plainText, err := shared.DecryptAesGcm(msg.Content, masterKey[:])
	if err != nil {
		fmt.Println("[ERROR] Failed decrypting message:", err)
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
