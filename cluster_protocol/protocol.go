package cluster_protocol

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"pqgch/gake"
	"pqgch/shared"
)

type CryptoSession struct {
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

func MakeSession(config shared.ConfigAccessor) CryptoSession {
	return CryptoSession{
		Xs:          make([][gake.SsLen]byte, len(config.GetNamesOrAddrs())),
		OnSharedKey: func() {},
		Commitments: make([]gake.Commitment, len(config.GetNamesOrAddrs())),
		Coins:       make([][gake.CoinLen]byte, len(config.GetNamesOrAddrs())),
	}
}

type Session struct {
	transport      shared.Transport
	config         shared.ConfigAccessor
	session        CryptoSession
	keyCiphertext  []byte
	mainSessionKey [32]byte
	Received       chan shared.Message
}

func NewSession(transport shared.Transport, config shared.ConfigAccessor) *Session {
	s := &Session{
		transport: transport,
		session:   MakeSession(config),
		config:    config,
		Received:  make(chan shared.Message, 10),
	}

	s.session.OnSharedKey = func() {
		if s.keyCiphertext == nil {
			fmt.Println("[CRYPTO] No key ciphertext, skipping")
			return
		}
		s.getMasterKey(s.keyCiphertext)
	}

	transport.SetMessageHandler(s.handleMessage)

	return s
}

func NewLeaderSession(transport shared.Transport, config shared.ConfigAccessor, keyRef *[32]byte) *Session {
	s := &Session{
		transport: transport,
		session:   MakeSession(config),
		config:    config,
	}

	s.session.OnSharedKey = func() {
		fmt.Println("[CRYPTO] Broadcasting main session key to cluster")
		keyMsg := shared.EncryptAndHMAC(keyRef[:], config, s.session.SharedSecret[:])
		s.transport.Send(keyMsg)
	}

	transport.SetMessageHandler(s.handleMessage)

	return s
}

func (s *Session) Init() {
	msg := getAkeAMsg(&s.session, s.config)
	s.transport.Send(msg)
}

func (s *Session) akeA(msg shared.Message) {
	akeB, xi := handleAkeA(msg, s.config, &s.session)
	s.transport.Send(akeB)
	if !xi.IsEmpty() {
		s.transport.Send(xi)
	}
}

func (s *Session) akeB(msg shared.Message) {
	xi := handleAkeB(msg, s.config, &s.session)
	if !xi.IsEmpty() {
		s.transport.Send(xi)
	}
}

func (s *Session) xi(msg shared.Message) {
	handleXi(msg, s.config, &s.session)
}

func (s *Session) keyHandler(msg shared.Message) {
	decodedContent, err := base64.StdEncoding.DecodeString(msg.Content)
	if err != nil {
		fmt.Printf("[ERROR] Failed to decode key message: %v\n", err)
		return
	}
	var wrappingKey [64]byte
	if s.session.SharedSecret == [gake.SsLen]byte{} {
		fmt.Println("[CRYPTO] GAKE handshake not complete; storing key message")
		s.keyCiphertext = decodedContent
		return
	}
	copy(wrappingKey[:], s.session.SharedSecret[:])
	recoveredKey, err := shared.DecryptAndCheckHMAC(decodedContent, wrappingKey[:])
	if err != nil {
		fmt.Println("[ERROR] Failed decrypting key message:", err)
		return
	}
	copy(s.mainSessionKey[:], recoveredKey)
	fmt.Printf("[CRYPTO] Main session established: %02x\n", s.mainSessionKey[:4])
}

func (s *Session) getMasterKey(decodedContent []byte) {
	recoveredKey, err := shared.DecryptAndCheckHMAC(decodedContent, s.session.SharedSecret[:])

	if err != nil {
		fmt.Println("[ERROR] Failed decrypting key message:", err)
		return
	}

	copy(s.mainSessionKey[:], recoveredKey)
	fmt.Printf("[CRYPTO] Main session established: %02x\n", s.mainSessionKey[:4])
}

func (s *Session) text(msg shared.Message) {
	plainText, err := shared.DecryptAesGcm(msg.Content, s.mainSessionKey[:])
	if err != nil {
		fmt.Println("[ERROR] Failed decrypting message:", err)
		return
	}
	msg.Content = plainText
	s.Received <- msg
}

func (s *Session) handleMessage(msg shared.Message) {
	switch msg.Type {
	case shared.AkeAMsg:
		s.akeA(msg)
	case shared.AkeBMsg:
		s.akeB(msg)
	case shared.XiMsg:
		s.xi(msg)
	case shared.KeyMsg:
		s.keyHandler(msg)
	default:
		s.text(msg)
	}
}

func (s *Session) SendText(text string) {
	if [32]byte(s.mainSessionKey) == [32]byte{} {
		fmt.Println("[CRYPTO] No master key available, skipping")
		return
	}
	cipherText, err := shared.EncryptAesGcm(text, s.mainSessionKey[:])
	if err != nil {
		fmt.Printf("[ERROR] Encryption failed: %v\n", err)
		return
	}
	msg := shared.Message{
		ID:         shared.GenerateUniqueID(),
		SenderID:   s.config.GetIndex(),
		SenderName: s.config.GetName(),
		Content:    cipherText,
		Type:       shared.TextMsg,
		ClusterID:  s.config.GetIndex(),
	}
	s.transport.Send(msg)
}

func handleAkeA(
	msg shared.Message,
	config shared.ConfigAccessor,
	session *CryptoSession,
) (shared.Message, shared.Message) {
	akeB := getAkeBMsg(session, msg, config)
	xi := checkLeftRightKeys(session, config)
	return akeB, xi
}

func handleAkeB(
	msg shared.Message,
	config shared.ConfigAccessor,
	session *CryptoSession,
) shared.Message {
	akeSendB, _ := base64.StdEncoding.DecodeString(msg.Content)
	session.KeyRight = gake.KexAkeSharedA(akeSendB, session.TkRight, session.EskaRight, config.GetDecodedSecretKey())
	xi := checkLeftRightKeys(session, config)
	return xi
}

func handleXi(
	msg shared.Message,
	config shared.ConfigAccessor,
	session *CryptoSession,
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
func checkLeftRightKeys(session *CryptoSession, config shared.ConfigAccessor) shared.Message {
	if session.KeyRight != [gake.SsLen]byte{} && session.KeyLeft != [gake.SsLen]byte{} {
		fmt.Println("[CRYPTO] Established shared keys with both neighbors")
		xcmMsg := getXiCommitmentCoinMsg(session, config)
		tryFinalizeProtocol(session, config)
		return xcmMsg
	}

	return shared.Message{}
}

func getXiCommitmentCoinMsg(session *CryptoSession, config shared.ConfigAccessor) shared.Message {
	xi, coin, commitment := computeXiCommitment(
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

	msg := shared.Message{
		ID:         shared.GenerateUniqueID(),
		SenderID:   config.GetIndex(),
		SenderName: config.GetName(),
		Type:       config.GetMessageType(shared.XiMsg),
		Content:    base64.StdEncoding.EncodeToString(buffer.Bytes()),
	}

	return msg
}

func computeXiCommitment(
	i int,
	key_right [32]byte,
	key_left [32]byte,
	public_key [1184]byte) ([32]byte, [44]byte, gake.Commitment) {
	var xi_i [36]byte
	var buf_int [4]byte

	buf_int[0] = byte(i >> 24)
	buf_int[1] = byte(i >> 16)
	buf_int[2] = byte(i >> 8)
	buf_int[3] = byte(i)

	xi := gake.XorKeys(key_right, key_left)
	ri := gake.GetRi()

	copy(xi_i[:], xi[:])
	copy(xi_i[32:], buf_int[:])

	commitment := gake.Commit_pke(public_key, xi_i, ri)

	return xi, ri, commitment
}

func tryFinalizeProtocol(session *CryptoSession, config shared.ConfigAccessor) {
	for i := range session.Xs {
		if (session.Xs)[i] == [gake.SsLen]byte{} {
			return
		}
	}

	fmt.Println("[CRYPTO] Received all Xs")

	ok := shared.CheckXs(session.Xs, len(config.GetNamesOrAddrs()))
	if ok {
		fmt.Println("[CRYPTO] Xs check: success")
	} else {
		fmt.Println("[CRYPTO] Xs check: fail")
		os.Exit(1)
	}

	ok = checkCommitments(len(config.GetNamesOrAddrs()), session.Xs, config.GetDecodedPublicKeys(), session.Coins, session.Commitments)
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

	otherLeftKeys := shared.ComputeAllLeftKeys(len(config.GetNamesOrAddrs()), config.GetIndex(), session.KeyLeft, session.Xs, pids)
	sharedSecret, sessionId := computeSessionKeyAndID(otherLeftKeys, pids, len(config.GetNamesOrAddrs()))

	fmt.Printf("[CRYPTO] SharedSecret%d: %02x...\n", config.GetIndex(), sharedSecret[:4])
	fmt.Printf("[CRYPTO] SessionId%d: %02x...\n", config.GetIndex(), sessionId[:4])

	session.SharedSecret = sharedSecret
	session.OnSharedKey()
}

// Recalculate the commitments and compare them to the received ones.
// If they do not match, it is an error and the protocol stops.
func checkCommitments(
	numParties int,
	xs [][32]byte,
	public_keys [][1184]byte,
	coins [][44]byte,
	commitments []gake.Commitment) bool {
	for i := range numParties {
		var xi_i [36]byte
		var buf_int [4]byte

		buf_int[0] = byte(i >> 24)
		buf_int[1] = byte(i >> 16)
		buf_int[2] = byte(i >> 8)
		buf_int[3] = byte(i)

		copy(xi_i[:32], xs[i][:])
		copy(xi_i[32:], buf_int[:])

		commitment := gake.Commit_pke(public_keys[i], xi_i, coins[i])

		for j := range 1088 {
			if commitment.CipherTextKem[j] != commitments[i].CipherTextKem[j] {
				return false
			}
		}

		for j := range 36 {
			if commitment.CipherTextDem[j] != commitments[i].CipherTextDem[j] {
				return false
			}
		}

		for j := range 16 {
			if commitment.Tag[j] != commitments[i].Tag[j] {
				return false
			}
		}
	}

	return true
}

// We define the master key as the concatenation of all the numParties left keys, together with party identifiers.
// Then, we hash the master key with SHA3-512 to obtain the 32 byte session key and 32 byte session ID.
func computeSessionKeyAndID(otherLeftKeys [][32]byte, pids [][20]byte, numParties int) ([32]byte, [32]byte) {
	masterKey := make([]byte, 52*numParties)

	for i := range otherLeftKeys {
		copy(masterKey[i*32:(i+1)*32], otherLeftKeys[i][:])
	}

	for i := range pids {
		copy(masterKey[len(otherLeftKeys)*32+i*20:len(otherLeftKeys)*32+(i+1)*20], pids[i][:])
	}

	sksid := gake.Sha3_512(masterKey)

	var sessionKey [32]byte
	var sessionId [32]byte

	copy(sessionKey[:], sksid[:32])
	copy(sessionId[:], sksid[32:])

	return sessionKey, sessionId
}

func getAkeAMsg(session *CryptoSession, config shared.ConfigAccessor) shared.Message {
	var rightIndex = (config.GetIndex() + 1) % len(config.GetNamesOrAddrs())
	var akeSendARight []byte
	akeSendARight, session.TkRight, session.EskaRight = gake.KexAkeInitA(config.GetDecodedPublicKey(rightIndex))

	msg := shared.Message{
		ID:         shared.GenerateUniqueID(),
		SenderID:   config.GetIndex(),
		SenderName: config.GetName(),
		Type:       config.GetMessageType(shared.AkeAMsg),
		ReceiverID: rightIndex,
		Content:    base64.StdEncoding.EncodeToString(akeSendARight),
	}

	return msg
}

func getAkeBMsg(session *CryptoSession, msg shared.Message, config shared.ConfigAccessor) shared.Message {
	akeSendA, _ := base64.StdEncoding.DecodeString(msg.Content)

	var akeSendB []byte
	akeSendB, session.KeyLeft = gake.KexAkeSharedB(
		akeSendA,
		config.GetDecodedSecretKey(),
		config.GetDecodedPublicKey(msg.SenderID))

	fmt.Println("[CRYPTO] Established shared key with left neighbor")

	msg = shared.Message{
		ID:         shared.GenerateUniqueID(),
		SenderID:   config.GetIndex(),
		SenderName: config.GetName(),
		Type:       config.GetMessageType(shared.AkeBMsg),
		ReceiverID: msg.SenderID,
		Content:    base64.StdEncoding.EncodeToString(akeSendB),
	}

	return msg
}
