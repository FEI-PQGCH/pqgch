package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"pqgch/gake"
)

// XOR all the Xs together. The result should be the zero byte array.
// If it is not, abort the protocol.
func CheckXs(xs [][32]byte, numParties int) bool {
	var check [32]byte
	copy(check[:], xs[0][:])

	for i := range numParties - 1 {
		check = gake.XorKeys(xs[i+1], check)
	}

	for i := range 32 {
		if check[i] != 0 {
			return false
		}
	}

	return true
}

// Compute all the left keys of all the protocol participants.
func ComputeAllLeftKeys(numParties int, partyIndex int, keyLeft [32]byte, xs [][32]byte, pids [][20]byte) [][32]byte {
	otherLeftKeys := make([][32]byte, numParties)  // Left keys of the other protocol participants.
	copy(otherLeftKeys[partyIndex][:], keyLeft[:]) // We already know our keyLeft.

	// Here, we compute the numParties-1 left keys of participant (partyIndex-j) mod numParties for j = 1..n-1.
	// These are the left keys of every other participant.
	// We can compute them using the Xs.
	for j := 1; j < numParties; j++ {
		var otherLeftKey [32]byte
		copy(otherLeftKey[:], keyLeft[:])

		for x := range j {
			var index = gake.Mod(partyIndex-x-1, numParties)
			otherLeftKey = gake.XorKeys(otherLeftKey, xs[index])
		}

		var index = gake.Mod(partyIndex-j, numParties)
		copy(otherLeftKeys[index][:], otherLeftKey[:])
	}

	return otherLeftKeys
}

func EncryptAesGcm(plaintext string, key []byte) (string, error) {
	var aesKey []byte
	switch gake.KyberK {
	case 2:
		aesKey = key[:16]
	case 3:
		aesKey = key[:24]
	case 4:

		aesKey = key[:32]
	default:
		return "", fmt.Errorf("unsupported kyber ID: %d", gake.KyberK)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	cipherText := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)

	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func DecryptAesGcm(encryptedText string, key []byte) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	var aesKey []byte
	switch gake.KyberK {
	case 2:
		aesKey = key[:16]
	case 3:
		aesKey = key[:24]
	case 4:
		aesKey = key[:32]
	default:
		return "", fmt.Errorf("unsupported kyber ID: %d", gake.KyberK)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(cipherText) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, cipherText := cipherText[:nonceSize], cipherText[nonceSize:]

	plainText, err := aesGCM.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}

	return string(plainText), nil
}

// mainSessionKey is 32 bytes. We encrypt the mainSessionKey by XOR-ing it with the first 32 bytes of the clusterSessionKey.
// Then, we create the MAC using the other 32 bytes of the clusterSessionKey.
func EncryptAndHMAC(mainSessionKey [gake.SsLen]byte, sender string, clusterSessionKey [gake.SsLen * 2]byte) Message {
	ciphertext := make([]byte, gake.SsLen)
	for i := range gake.SsLen {
		ciphertext[i] = mainSessionKey[i] ^ clusterSessionKey[i]
	}
	hmacKey := clusterSessionKey[gake.SsLen:]
	mac := hmac.New(sha256.New, hmacKey)
	mac.Write(ciphertext)
	tag := mac.Sum(nil)
	ciphertext = append(ciphertext, tag...)
	msg := Message{
		ID:         UniqueID(),
		SenderID:   -1,
		SenderName: sender,
		Type:       KeyMsg,
		Content:    base64.StdEncoding.EncodeToString(ciphertext),
	}
	return msg
}

func DecryptAndCheckHMAC(encryptedText []byte, key [64]byte) ([]byte, error) {
	ciphertext := encryptedText[:gake.SsLen]
	tag := encryptedText[gake.SsLen:]
	hmacKey := key[gake.SsLen:]
	mac := hmac.New(sha256.New, hmacKey)
	mac.Write(ciphertext)
	expectedTag := mac.Sum(nil)
	if !hmac.Equal(tag, expectedTag) {
		return nil, errors.New("tag mismatch")
	}
	recoveredKey := make([]byte, gake.SsLen)
	for i := range gake.SsLen {
		recoveredKey[i] = key[i] ^ ciphertext[i]
	}
	return recoveredKey, nil
}
