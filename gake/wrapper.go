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

type KemKeyPair struct {
	Pk [1184]byte
	Sk [2400]byte
}

type Party struct {
	Pk          [1184]byte
	Sk          [2400]byte
	KeyLeft     [32]byte
	KeyRight    [32]byte
	Xs          [][32]byte
	Coins       [][44]byte
	Commitments []Commitment
	MasterKey   [][32]byte
}

type Commitment struct {
	CipherTextKem [1088]byte
	CipherTextDem [36]byte
	Tag           [16]byte
}

func GetKemKeyPair() KemKeyPair {
	var pk [1184]byte
	var sk [2400]byte

	C.crypto_kem_keypair(
		(*C.uchar)(unsafe.Pointer(&pk[0])),
		(*C.uchar)(unsafe.Pointer(&sk[0])))

	return KemKeyPair{pk, sk}
}

func KexAkeInitA(pkb [1184]byte) ([]byte, []byte, []byte) {
	var ake_senda [2272]byte
	var tk [32]byte
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
	var kb [32]byte

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

func ComputeXsCommitment(
	i int,
	key_right [32]byte,
	key_left [32]byte,
	public_key [1184]byte) ([32]byte, [44]byte, Commitment) {
	var xi [32]byte
	var coin [44]byte
	var commitment Commitment

	var msg [36]byte
	var buf_int [4]byte

	buf_int[0] = byte(i >> 24)
	buf_int[1] = byte(i >> 16)
	buf_int[2] = byte(i >> 8)
	buf_int[3] = byte(i)

	C.xor_keys(
		(*C.uchar)(unsafe.Pointer(&key_right[0])),
		(*C.uchar)(unsafe.Pointer(&key_left[0])),
		(*C.uchar)(unsafe.Pointer(&xi[0])))

	C.randombytes(
		(*C.uchar)(unsafe.Pointer(&coin[0])),
		44)

	copy(msg[:], xi[:])
	copy(msg[32:], buf_int[:])

	C.commit(
		(*C.uchar)(unsafe.Pointer(&public_key[0])),
		(*C.uchar)(unsafe.Pointer(&msg[0])),
		36,
		(*C.uchar)(unsafe.Pointer(&coin[0])),
		(*C.Commitment)(unsafe.Pointer(&commitment)))

	return xi, coin, commitment
}

func CheckXs(xs [][32]byte, numParties int) bool {
	zero := make([]byte, 32)
	check := make([]byte, 32)

	copy(check[:], xs[0][:])
	for i := 0; i < numParties-1; i++ {
		C.xor_keys(
			(*C.uchar)(unsafe.Pointer(&xs[i+1][0])),
			(*C.uchar)(unsafe.Pointer(&check[0])),
			(*C.uchar)(unsafe.Pointer(&check[0])),
		)
	}

	for i := 0; i < 32; i++ {
		if check[i] != zero[i] {
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
	for i := 0; i < numParties; i++ {
		msg := make([]byte, 32+4)

		var buf_int [4]byte
		buf_int[0] = byte(i >> 24)
		buf_int[1] = byte(i >> 16)
		buf_int[2] = byte(i >> 8)
		buf_int[3] = byte(i)

		copy(msg[:32], xs[i][:])
		copy(msg[32:], buf_int[:])

		var commitment Commitment

		C.commit(
			(*C.uchar)(unsafe.Pointer(&public_keys[i][0])),
			(*C.uchar)(unsafe.Pointer(&msg[0])),
			36,
			(*C.uchar)(unsafe.Pointer(&coins[i][0])),
			(*C.Commitment)(unsafe.Pointer(&commitment)))

		for j := 0; j < 1088; j++ {
			if commitment.CipherTextKem[j] != commitments[i].CipherTextKem[j] {
				return false
			}
		}

		for j := 0; j < 36; j++ {
			if commitment.CipherTextDem[j] != commitments[i].CipherTextDem[j] {
				return false
			}
		}

		for j := 0; j < 16; j++ {
			if commitment.Tag[j] != commitments[i].Tag[j] {
				return false
			}
		}
	}

	return true
}

func ComputeMasterKey(numParties int, partyIndex int, key_left [32]byte, xs [][32]byte) [][32]byte {
	masterKey := make([][32]byte, numParties)

	copy(masterKey[partyIndex][:], key_left[:])

	for i := 1; i < numParties; i++ {
		var mk [32]byte
		copy(mk[:], key_left[:])

		for j := 0; j < i; j++ {
			var index = C.mod(C.int(partyIndex-j-1), C.int(numParties))

			C.xor_keys(
				(*C.uchar)(unsafe.Pointer(&mk[0])),
				(*C.uchar)(unsafe.Pointer(&xs[index][0])),
				(*C.uchar)(unsafe.Pointer(&mk[0])))
		}

		var index = C.mod(C.int(partyIndex-i), C.int(numParties))

		copy(masterKey[index][:], mk[:])
	}

	return masterKey
}

func ComputeSkSid(numParties int, masterKeys [][32]byte, pids [][20]byte) [64]byte {
	mki := make([]byte, 52*numParties)

	C.concat_masterkey(
		(*C.MasterKey)(unsafe.Pointer(&masterKeys[0][0])),
		(*C.Pid)(unsafe.Pointer(&pids[0][0])),
		C.int(numParties),
		(*C.uchar)(unsafe.Pointer(&mki[0])),
	)

	var sksid [64]byte

	C.hash_g_fn(
		(*C.uchar)(unsafe.Pointer(&sksid[0])),
		(*C.uchar)(unsafe.Pointer(&mki[0])),
		64)

	return sksid
}
