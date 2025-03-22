package leader_protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"pqgch/gake"
	"pqgch/shared"
	"slices"
)

type CryptoLeaderSession struct {
	TkRight      []byte
	EskaRight    []byte
	KeyLeft      [gake.SsLen]byte
	KeyRight     [gake.SsLen]byte
	Xs           [][gake.SsLen]byte
	Commitments  [][32]byte
	Coins        [][gake.CoinLen]byte
	SharedSecret [gake.SsLen]byte
	OnSharedKey  func()
}

func NewCryptoLeaderSession(config shared.ConfigAccessor) CryptoLeaderSession {
	return CryptoLeaderSession{
		Xs:          make([][gake.SsLen]byte, len(config.GetNamesOrAddrs())),
		OnSharedKey: func() {},
		Commitments: make([][32]byte, len(config.GetNamesOrAddrs())),
		Coins:       make([][gake.CoinLen]byte, len(config.GetNamesOrAddrs())),
	}
}

type LeaderSession struct {
	transport shared.Transport
	config    shared.ConfigAccessor
	session   CryptoLeaderSession
}

func NewLeaderSession(transport shared.Transport, config shared.ConfigAccessor) *LeaderSession {
	s := &LeaderSession{
		transport: transport,
		session:   NewCryptoLeaderSession(config),
		config:    config,
	}

	transport.SetMessageHandler(s.handleMessage)

	return s
}

func (s *LeaderSession) Init() {
	msg := getAkeAMsgLeader(&s.session, s.config)
	s.transport.Send(msg)
}

func (s *LeaderSession) GetKeyRef() *[32]byte {
	return &s.session.SharedSecret
}

func (s *LeaderSession) akeALeader(msg shared.Message) {
	akeB, xi := handleAkeALeader(msg, s.config, &s.session)
	s.transport.Send(akeB)
	if !xi.IsEmpty() {
		s.transport.Send(xi)
	}
}

func (s *LeaderSession) akeBLeader(msg shared.Message) {
	xi := handleAkeBLeader(msg, s.config, &s.session)
	if !xi.IsEmpty() {
		s.transport.Send(xi)
	}
}

func (s *LeaderSession) xiLeader(msg shared.Message) {
	handleXiLeader(msg, s.config, &s.session)
}

func (s *LeaderSession) handleMessage(msg shared.Message) {
	switch msg.Type {
	case shared.LeaderAkeAMsg:
		s.akeALeader(msg)
	case shared.LeaderAkeBMsg:
		s.akeBLeader(msg)
	case shared.LeaderXiMsg:
		s.xiLeader(msg)
	default:
		fmt.Printf("[ERROR] Unknown message type encountered\n")
	}
}

func checkLeftRightKeysLeader(session *CryptoLeaderSession, config shared.ConfigAccessor) shared.Message {
	if session.KeyRight != [gake.SsLen]byte{} && session.KeyLeft != [gake.SsLen]byte{} {
		fmt.Println("[CRYPTO] Established shared keys with both neighbors")
		xcmMsg := getXiCommitmentCoinMsgLeader(session, config)
		tryFinalizeProtocolLeader(session, config)
		return xcmMsg
	}

	return shared.Message{}
}

func getXiCommitmentCoinMsgLeader(session *CryptoLeaderSession, config shared.ConfigAccessor) shared.Message {
	xi := gake.XorKeys(session.KeyRight, session.KeyLeft)
	ri := gake.GetRi()
	x := append(xi[:], ri[:]...)
	commitment := sha256.Sum256(x)

	session.Xs[config.GetIndex()] = xi
	session.Commitments[config.GetIndex()] = commitment
	session.Coins[config.GetIndex()] = ri

	var buffer bytes.Buffer
	buffer.Grow(gake.SsLen + gake.SsLen + gake.CoinLen)
	buffer.Write(xi[:])
	buffer.Write(commitment[:])
	buffer.Write(ri[:])

	msg := shared.Message{
		ID:         shared.GenerateUniqueID(),
		SenderID:   config.GetIndex(),
		SenderName: config.GetName(),
		Type:       config.GetMessageType(shared.XiMsg),
		Content:    base64.StdEncoding.EncodeToString(buffer.Bytes()),
	}

	return msg
}

func checkCommitmentsLeader(
	numParties int,
	xs [][32]byte,
	coins [][44]byte,
	commitments [][32]byte) bool {
	for i := range numParties {
		x := append(xs[i][:], coins[i][:]...)
		commitment := sha256.Sum256(x)

		for j := range 32 {
			if commitment[j] != commitments[i][j] {
				return false
			}
		}
	}

	return true
}

func tryFinalizeProtocolLeader(session *CryptoLeaderSession, config shared.ConfigAccessor) {
	if slices.Contains(session.Xs, [gake.SsLen]byte{}) {
		return
	}

	fmt.Println("[CRYPTO] Received all Xs")

	ok := gake.CheckXs(session.Xs, len(config.GetNamesOrAddrs()))
	if ok {
		fmt.Println("[CRYPTO] Xs check: success")
	} else {
		fmt.Println("[CRYPTO] Xs check: fail")
		os.Exit(1)
	}

	ok = checkCommitmentsLeader(len(config.GetNamesOrAddrs()), session.Xs, session.Coins, session.Commitments)
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

func getAkeAMsgLeader(session *CryptoLeaderSession, config shared.ConfigAccessor) shared.Message {
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

func getAkeBMsgLeader(session *CryptoLeaderSession, msg shared.Message, config shared.ConfigAccessor) shared.Message {
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

func handleAkeALeader(
	msg shared.Message,
	config shared.ConfigAccessor,
	session *CryptoLeaderSession,
) (shared.Message, shared.Message) {
	akeB := getAkeBMsgLeader(session, msg, config)
	xi := checkLeftRightKeysLeader(session, config)
	return akeB, xi
}

func handleAkeBLeader(
	msg shared.Message,
	config shared.ConfigAccessor,
	session *CryptoLeaderSession,
) shared.Message {
	akeSendB, _ := base64.StdEncoding.DecodeString(msg.Content)
	session.KeyRight = gake.KexAkeSharedA(akeSendB, session.TkRight, session.EskaRight, config.GetDecodedSecretKey())
	xi := checkLeftRightKeysLeader(session, config)
	return xi
}

func handleXiLeader(
	msg shared.Message,
	config shared.ConfigAccessor,
	session *CryptoLeaderSession,
) {
	if msg.SenderID == config.GetIndex() {
		return
	}

	decoded, _ := base64.StdEncoding.DecodeString(msg.Content)

	xi := decoded[:gake.SsLen]
	commitment := decoded[gake.SsLen : gake.SsLen+gake.SsLen]
	ri := decoded[gake.SsLen+gake.SsLen:]

	var xiArr [gake.SsLen]byte
	copy(xiArr[:], xi)
	var commitmentArr [32]byte
	copy(commitmentArr[:], commitment)
	var coinArr [gake.CoinLen]byte
	copy(coinArr[:], ri)

	session.Commitments[msg.SenderID] = commitmentArr
	session.Coins[msg.SenderID] = coinArr
	session.Xs[msg.SenderID] = xiArr
	tryFinalizeProtocolLeader(session, config)
}
