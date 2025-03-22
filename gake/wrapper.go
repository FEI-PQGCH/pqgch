package gake

/*
#cgo CFLAGS: -I./kyber-gake/ref -Wall -Wextra -Wpedantic -Werror -Wshadow -Wpointer-arith -O3 -fwrapv
#cgo LDFLAGS: -L./kyber-gake/ref -lssl -lcrypto

#include "gake.c"
#include "utils.c"
#include "kex.c"
#include "kem.c"
#include "commitment.c"
#include "fips202.c"
#include "indcca.c"
#include "indcpa.c"
#include "poly.c"
#include "polyvec.c"
#include "randombytes.c"
#include "ntt.c"
#include "reduce.c"
#include "kem_det.c"
#include "verify.c"
#include "cbd.c"
#include "symmetric-shake.c"
#include "aes256gcm.c"

void hash_g_fn(unsigned char *out, const unsigned char *in, unsigned long long inlen) {
    hash_g(out, in, inlen);
}
*/
import "C"
import (
	"unsafe"
)

const (
	PkLen    = 1184
	SkLen    = 2400
	CtKemLen = 1088
	CtDemLen = 36
	TagLen   = 16
	SsLen    = 32
	CoinLen  = 44
	PidLen   = 20
)

type KemKeyPair struct {
	Pk [PkLen]byte
	Sk [SkLen]byte
}

type Commitment struct {
	CipherTextKem [CtKemLen]byte
	CipherTextDem [CtDemLen]byte
	Tag           [TagLen]byte
}

// C Wrappers

func GetKemKeyPair() KemKeyPair {
	var pk [PkLen]byte
	var sk [SkLen]byte

	C.crypto_kem_keypair(
		(*C.uchar)(unsafe.Pointer(&pk[0])),
		(*C.uchar)(unsafe.Pointer(&sk[0])))

	return KemKeyPair{pk, sk}
}

func KexAkeInitA(pkb [PkLen]byte) ([]byte, []byte, []byte) {
	var ake_senda [2272]byte
	var tk [SsLen]byte
	var eska [2400]byte

	C.kex_ake_initA(
		(*C.uchar)(unsafe.Pointer(&ake_senda[0])),
		(*C.uchar)(unsafe.Pointer(&tk[0])),
		(*C.uchar)(unsafe.Pointer(&eska[0])),
		(*C.uchar)(unsafe.Pointer(&pkb[0])))

	return ake_senda[:], tk[:], eska[:]
}

func KexAkeSharedB(ake_senda []byte, skb []byte, pka [1184]byte) ([]byte, [32]byte) {
	var ake_sendb [2176]byte
	var kb [SsLen]byte

	C.kex_ake_sharedB(
		(*C.uchar)(unsafe.Pointer(&ake_sendb[0])),
		(*C.uchar)(unsafe.Pointer(&kb[0])),
		(*C.uchar)(unsafe.Pointer(&ake_senda[0])),
		(*C.uchar)(unsafe.Pointer(&skb[0])),
		(*C.uchar)(unsafe.Pointer(&pka[0])))

	return ake_sendb[:], kb
}

func KexAkeSharedA(ake_sendb []byte, tk []byte, eska []byte, ska []byte) [32]byte {
	var ka [32]byte

	C.kex_ake_sharedA(
		(*C.uchar)(unsafe.Pointer(&ka[0])),
		(*C.uchar)(unsafe.Pointer(&ake_sendb[0])),
		(*C.uchar)(unsafe.Pointer(&tk[0])),
		(*C.uchar)(unsafe.Pointer(&eska[0])),
		(*C.uchar)(unsafe.Pointer(&ska[0])))

	return ka
}

func XorKeys(key_right [32]byte, key_left [32]byte) [32]byte {
	var xi [32]byte

	C.xor_keys(
		(*C.uchar)(unsafe.Pointer(&key_right[0])),
		(*C.uchar)(unsafe.Pointer(&key_left[0])),
		(*C.uchar)(unsafe.Pointer(&xi[0])))

	return xi
}

func GetRi() [44]byte {
	var coin [44]byte

	C.randombytes(
		(*C.uchar)(unsafe.Pointer(&coin[0])),
		44)

	return coin
}

func sha3_512(x []byte) [64]byte {
	var out [64]byte

	C.hash_g_fn(
		(*C.uchar)(unsafe.Pointer(&out[0])),
		(*C.uchar)(unsafe.Pointer(&x[0])),
		(C.ulonglong)(len(x)))

	return out
}

func commit_pke(pk [PkLen]byte, xi_i [36]byte, ri [44]byte) Commitment {
	var commitment Commitment

	C.commit(
		(*C.uchar)(unsafe.Pointer(&pk[0])),
		(*C.uchar)(unsafe.Pointer(&xi_i[0])),
		36,
		(*C.uchar)(unsafe.Pointer(&ri[0])),
		(*C.Commitment)(unsafe.Pointer(&commitment)))

	return commitment
}

func mod(i int, j int) int {
	return int(C.mod(C.int(i), C.int(j)))
}

// Protocol logic

func ComputeXsCommitment(
	i int,
	key_right [32]byte,
	key_left [32]byte,
	public_key [1184]byte) ([32]byte, [44]byte, Commitment) {
	var xi_i [36]byte
	var buf_int [4]byte

	buf_int[0] = byte(i >> 24)
	buf_int[1] = byte(i >> 16)
	buf_int[2] = byte(i >> 8)
	buf_int[3] = byte(i)

	xi := XorKeys(key_right, key_left)
	ri := GetRi()

	copy(xi_i[:], xi[:])
	copy(xi_i[32:], buf_int[:])

	commitment := commit_pke(public_key, xi_i, ri)

	return xi, ri, commitment
}

func CheckXs(xs [][32]byte, numParties int) bool {
	var check [32]byte
	copy(check[:], xs[0][:])

	for i := range numParties - 1 {
		check = XorKeys(xs[i+1], check)
	}

	for i := range 32 {
		if check[i] != 0 {
			return false
		}
	}

	return true
}

func CheckCommitments(
	numParties int,
	xs [][32]byte,
	public_keys [][1184]byte,
	coins [][44]byte,
	commitments []Commitment) bool {
	for i := range numParties {
		var xi_i [36]byte
		var buf_int [4]byte

		buf_int[0] = byte(i >> 24)
		buf_int[1] = byte(i >> 16)
		buf_int[2] = byte(i >> 8)
		buf_int[3] = byte(i)

		copy(xi_i[:32], xs[i][:])
		copy(xi_i[32:], buf_int[:])

		commitment := commit_pke(public_keys[i], xi_i, coins[i])

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

func ComputeSharedKey(numParties int, partyIndex int, key_left [32]byte, xs [][32]byte, pids [][20]byte) ([32]byte, [32]byte) {
	masterKey := make([][32]byte, numParties)
	copy(masterKey[partyIndex][:], key_left[:])

	for i := 1; i < numParties; i++ {
		var mk [32]byte
		copy(mk[:], key_left[:])

		for j := range i {
			var index = mod(partyIndex-j-1, numParties)
			mk = XorKeys(mk, xs[index])
		}

		var index = mod(partyIndex-i, numParties)
		copy(masterKey[index][:], mk[:])
	}

	mki := concatMasterKey(masterKey, pids, numParties)
	sksid := sha3_512(mki)

	var sharedSecret [32]byte
	var sessionId [32]byte

	copy(sharedSecret[:], sksid[:32])
	copy(sessionId[:], sksid[32:])

	return sharedSecret, sessionId
}

func concatMasterKey(masterKeys [][32]byte, pids [][20]byte, numParties int) []byte {
	mki := make([]byte, 52*numParties)

	for i := range masterKeys {
		copy(mki[i*32:(i+1)*32], masterKeys[i][:])
	}

	for i := range pids {
		copy(mki[len(masterKeys)*32+i*20:len(masterKeys)*32+(i+1)*20], pids[i][:])
	}

	return mki
}
