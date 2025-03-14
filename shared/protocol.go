package shared

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"pqgch-client/gake"

	"github.com/google/uuid"
)

type Session struct {
	TkRight      []byte
	EskaRight    []byte
	KeyLeft      [gake.SsLen]byte
	KeyRight     [gake.SsLen]byte
	Xs           [][gake.SsLen]byte
	Commitments  []gake.Commitment
	Coins        [][gake.CoinLen]byte
	SharedSecret [gake.SsLen]byte
	OnSharedKey  func()
}

func MakeSession(config ConfigAccessor) Session {
	return Session{
		Xs:          make([][gake.SsLen]byte, len(config.GetNamesOrAddrs())),
		OnSharedKey: func() {},
		Commitments: make([]gake.Commitment, len(config.GetNamesOrAddrs())),
		Coins:       make([][gake.CoinLen]byte, len(config.GetNamesOrAddrs())),
	}
}

func checkLeftRightKeys(session *Session, config ConfigAccessor) Message {
	if session.KeyRight != [gake.SsLen]byte{} && session.KeyLeft != [gake.SsLen]byte{} {
		fmt.Println("[CRYPTO] Established shared keys with both neighbors")
		xcmMsg := getXiCommitmentCoinMsg(session, config)
		tryFinalizeProtocol(session, config)
		return xcmMsg
	}

	return Message{}
}

func getXiCommitmentCoinMsg(session *Session, config ConfigAccessor) Message {
	xi, coin, commitment := gake.ComputeXsCommitment(
		config.GetIndex(),
		session.KeyRight,
		session.KeyLeft,
		config.GetDecodedPublicKey(config.GetIndex()))

	session.Xs[config.GetIndex()] = xi
	session.Commitments[config.GetIndex()] = commitment
	session.Coins[config.GetIndex()] = coin

	var buffer bytes.Buffer
	buffer.Grow(gake.SsLen + gake.CtKemLen + gake.CtDemLen + gake.TagLen + gake.CoinLen)
	buffer.Write(xi[:])
	buffer.Write(commitment.CipherTextKem[:])
	buffer.Write(commitment.CipherTextDem[:])
	buffer.Write(commitment.Tag[:])
	buffer.Write(coin[:])

	msg := Message{
		ID:         uuid.New().String(),
		SenderID:   config.GetIndex(),
		SenderName: config.GetName(),
		Type:       config.GetMessageType(XiMsg),
		Content:    base64.StdEncoding.EncodeToString(buffer.Bytes()),
	}

	return msg
}

func tryFinalizeProtocol(session *Session, config ConfigAccessor) {
	for i := range session.Xs {
		if (session.Xs)[i] == [gake.SsLen]byte{} {
			return
		}
	}

	fmt.Println("[CRYPTO] Received all Xs")

	ok := gake.CheckXs(session.Xs, len(config.GetNamesOrAddrs()))
	if ok {
		fmt.Println("[CRYPTO] Xs check: success")
	} else {
		fmt.Println("[CRYPTO] Xs check: fail")
		os.Exit(1)
	}

	ok = gake.CheckCommitments(len(config.GetNamesOrAddrs()), session.Xs, config.GetDecodedPublicKeys(), session.Coins, session.Commitments)
	if ok {
		fmt.Println("[CRYPTO] Commitments check: success")
	} else {
		fmt.Println("[CRYPTO] Commitments check: fail")
		os.Exit(1)
	}

	for i := range session.Xs {
		fmt.Printf("[CRYPTO] X%d: %02x\n", i, (session.Xs)[i][:4])
	}

	pids := make([][gake.PidLen]byte, len(config.GetNamesOrAddrs()))
	stringPids := config.GetNamesOrAddrs()
	for i := range config.GetNamesOrAddrs() {
		var byteArr [gake.PidLen]byte
		copy(byteArr[:], []byte(stringPids[i]))
		pids[i] = byteArr
	}

	sharedSecret, sessioId := gake.ComputeSharedKey(len(config.GetNamesOrAddrs()), config.GetIndex(), session.KeyLeft, session.Xs, pids)
	fmt.Printf("[CRYPTO] SharedSecret%d: %02x...\n", config.GetIndex(), sharedSecret[:4])
	fmt.Printf("[CRYPTO] SessionId%d: %02x...\n", config.GetIndex(), sessioId[:4])

	session.SharedSecret = sharedSecret
	session.OnSharedKey()
}

func GetAkeAMsg(session *Session, config ConfigAccessor) Message {
	var rightIndex = (config.GetIndex() + 1) % len(config.GetNamesOrAddrs())
	var akeSendARight []byte
	akeSendARight, session.TkRight, session.EskaRight = gake.KexAkeInitA(config.GetDecodedPublicKey(rightIndex))

	msg := Message{
		ID:         uuid.New().String(),
		SenderID:   config.GetIndex(),
		SenderName: config.GetName(),
		Type:       config.GetMessageType(AkeAMsg),
		ReceiverID: rightIndex,
		Content:    base64.StdEncoding.EncodeToString(akeSendARight),
	}

	return msg
}

func getAkeBMsg(session *Session, msg Message, config ConfigAccessor) Message {
	akeSendA, _ := base64.StdEncoding.DecodeString(msg.Content)

	var akeSendB []byte
	akeSendB, session.KeyLeft = gake.KexAkeSharedB(
		akeSendA,
		config.GetDecodedSecretKey(),
		config.GetDecodedPublicKey(msg.SenderID))

	fmt.Println("[CRYPTO] Established shared key with left neighbor")

	msg = Message{
		ID:         uuid.New().String(),
		SenderID:   config.GetIndex(),
		SenderName: config.GetName(),
		Type:       config.GetMessageType(AkeBMsg),
		ReceiverID: msg.SenderID,
		Content:    base64.StdEncoding.EncodeToString(akeSendB),
	}

	return msg
}

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
		ID:         uuid.New().String(),
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

func HandleAkeA(
	msg Message,
	config ConfigAccessor,
	session *Session,
) (Message, Message) {
	akeB := getAkeBMsg(session, msg, config)
	xi := checkLeftRightKeys(session, config)
	return akeB, xi
}

func HandleAkeB(
	msg Message,
	config ConfigAccessor,
	session *Session,
) Message {
	akeSendB, _ := base64.StdEncoding.DecodeString(msg.Content)
	session.KeyRight = gake.KexAkeSharedA(akeSendB, session.TkRight, session.EskaRight, config.GetDecodedSecretKey())
	xi := checkLeftRightKeys(session, config)
	return xi
}

func HandleXi(
	msg Message,
	config ConfigAccessor,
	session *Session,
) {
	if msg.SenderID == config.GetIndex() {
		return
	}

	decoded, _ := base64.StdEncoding.DecodeString(msg.Content)

	xi := decoded[:gake.SsLen]
	kem := decoded[gake.SsLen : gake.SsLen+gake.CtKemLen]
	dem := decoded[gake.SsLen+gake.CtKemLen : gake.SsLen+gake.CtKemLen+gake.CtDemLen]
	tag := decoded[gake.SsLen+gake.CtKemLen+gake.CtDemLen : gake.SsLen+gake.CtKemLen+gake.CtDemLen+gake.TagLen]
	coin := decoded[gake.SsLen+gake.CtKemLen+gake.CtDemLen+gake.TagLen:]

	var xiArr [gake.SsLen]byte
	copy(xiArr[:], xi)
	var kemArr [gake.CtKemLen]byte
	copy(kemArr[:], kem)
	var demArr [gake.CtDemLen]byte
	copy(demArr[:], dem)
	var tagArr [gake.TagLen]byte
	copy(tagArr[:], tag)
	var coinArr [gake.CoinLen]byte
	copy(coinArr[:], coin)

	var commitment gake.Commitment
	commitment.CipherTextDem = demArr
	commitment.CipherTextKem = kemArr
	commitment.Tag = tagArr

	session.Commitments[msg.SenderID] = commitment
	session.Coins[msg.SenderID] = coinArr
	session.Xs[msg.SenderID] = xiArr
	tryFinalizeProtocol(session, config)
}

func LoadClusterKey(filePath string) ([gake.SsLen]byte, error) {
	var key [gake.SsLen]byte

	data, err := os.ReadFile(filePath)
	if err != nil {
		return key, err
	}

	decoded, err := hex.DecodeString(string(data))
	if err != nil {
		return key, err
	}
	if len(decoded) < gake.SsLen {
		return key, errors.New("cluster key file is too short")
	}
	copy(key[:], decoded)
	return key, nil
}
