package shared

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"pqgch/gake"
)

func EncryptAesGcm(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
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

	block, err := aes.NewCipher(key)
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

func EncryptAndHMAC(masterKey []byte, config ConfigAccessor, key []byte) Message {
	ciphertext := make([]byte, gake.SsLen)
	for i := range gake.SsLen {
		ciphertext[i] = masterKey[i] ^ key[i]
	}
	hmacKey := key[gake.SsLen:]
	mac := hmac.New(sha256.New, hmacKey)
	mac.Write(ciphertext)
	tag := mac.Sum(nil)
	ciphertext = append(ciphertext, tag...)
	msg := Message{
		ID:         GenerateUniqueID(),
		SenderID:   -1,
		SenderName: config.GetName(),
		Type:       KeyMsg,
		Content:    base64.StdEncoding.EncodeToString(ciphertext),
	}
	return msg
}

func DecryptAndCheckHMAC(encryptedText []byte, key []byte) ([]byte, error) {
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
