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
func CheckXs(xs [][gake.SsLen]byte, numParties int) bool {
	var check [gake.SsLen]byte
	copy(check[:], xs[0][:])

	for i := range numParties - 1 {
		check = gake.XorKeys(xs[i+1], check)
	}

	for i := range gake.SsLen {
		if check[i] != 0 {
			return false
		}
	}

	return true
}

// Compute all the left keys of all the protocol participants.
func ComputeAllLeftKeys(numParties int, partyIndex int, keyLeft [gake.SsLen]byte, xs [][gake.SsLen]byte, pids [][gake.PidLen]byte) [][gake.SsLen]byte {
	otherLeftKeys := make([][gake.SsLen]byte, numParties) // Left keys of the other protocol participants.
	copy(otherLeftKeys[partyIndex][:], keyLeft[:])        // We already know our keyLeft.

	// Here, we compute the numParties-1 left keys of participant (partyIndex-j) mod numParties for j = 1..n-1.
	// These are the left keys of every other participant.
	// We can compute them using the Xs.
	for j := 1; j < numParties; j++ {
		var otherLeftKey [gake.SsLen]byte
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

// Encrypt and HMAC the Main Session Key with the Cluster Session Key for transpot to the cluster members.
func EncryptAndHMAC(mainSessionKey [gake.SsLen]byte, sender string, clusterSessionKey [2 * gake.SsLen]byte) (Message, error) {
	maskingKey, hmacKey, err := splitAndCheckKey(clusterSessionKey)
	if err != nil {
		return Message{}, err
	}

	ciphertext := make([]byte, gake.SsLen)
	for i := range gake.SsLen {
		ciphertext[i] = mainSessionKey[i] ^ maskingKey[i]
	}

	mac := hmac.New(sha256.New, hmacKey[:])
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
	return msg, nil
}

// Decrypt and check HMAC of the received Main Session Key ciphertext using the Cluster Session Key.
func DecryptAndCheckHMAC(encryptedMainSessionKey []byte, clusterSessionKey [2 * gake.SsLen]byte) ([]byte, error) {
	maskingKey, hmacKey, err := splitAndCheckKey(clusterSessionKey)
	if err != nil {
		return nil, err
	}

	ciphertext := encryptedMainSessionKey[:gake.SsLen]
	tag := encryptedMainSessionKey[gake.SsLen:]
	mac := hmac.New(sha256.New, hmacKey[:])
	mac.Write(ciphertext)
	expectedTag := mac.Sum(nil)

	if !hmac.Equal(tag, expectedTag) {
		return nil, errors.New("tag mismatch")
	}

	mainSessionKey := make([]byte, gake.SsLen)
	for i := range gake.SsLen {
		mainSessionKey[i] = maskingKey[i] ^ ciphertext[i]
	}
	return mainSessionKey, nil
}

func splitAndCheckKey(key [2 * gake.SsLen]byte) ([gake.SsLen]byte, [gake.SsLen]byte, error) {
	var maskingKey, hmacKey [gake.SsLen]byte
	copy(maskingKey[:], key[:gake.SsLen])
	copy(hmacKey[:], key[gake.SsLen:])

	if maskingKey == [gake.SsLen]byte{} || hmacKey == [gake.SsLen]byte{} {
		return [gake.SsLen]byte{}, [gake.SsLen]byte{}, errors.New("nil key")
	}
	return maskingKey, hmacKey, nil
}
