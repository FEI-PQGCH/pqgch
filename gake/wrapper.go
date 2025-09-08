package gake

/*
#cgo CFLAGS: -I./kyber-gake/ref -Wall  -Wextra -Wpedantic -Wshadow -Wpointer-arith -O3
#cgo LDFLAGS: -L./kyber-gake/ref -lssl -lcrypto

#include "params.h"

enum {
    GoKyberK    = KYBER_K,
    GoPkLen     = KYBER_PUBLICKEYBYTES,
    GoSkLen     = KYBER_SECRETKEYBYTES,
    GoCtKemLen  = KYBER_CIPHERTEXTBYTES,
    GoSsBytes   = KYBER_SSBYTES
};

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
	KyberK   = int(C.GoKyberK)
	PkLen    = int(C.GoPkLen)
	SkLen    = int(C.GoSkLen)
	CtKemLen = int(C.GoCtKemLen)
	SsLen    = int(C.GoSsBytes)
	CtDemLen = 36
	TagLen   = 16
	CoinLen  = 44
	PidLen   = 20
	AkeSendB = 2 * CtKemLen
	AkeSendA = PkLen + CtKemLen
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

func GetKemKeyPair() KemKeyPair {
	var pk [PkLen]byte
	var sk [SkLen]byte

	C.crypto_kem_keypair(
		(*C.uchar)(unsafe.Pointer(&pk[0])),
		(*C.uchar)(unsafe.Pointer(&sk[0])))

	return KemKeyPair{pk, sk}
}

func KexAkeInitA(pkb [PkLen]byte) ([]byte, []byte, []byte) {
	var ake_senda [AkeSendA]byte
	var tk [SsLen]byte
	var eska [SkLen]byte

	C.kex_ake_initA(
		(*C.uchar)(unsafe.Pointer(&ake_senda[0])),
		(*C.uchar)(unsafe.Pointer(&tk[0])),
		(*C.uchar)(unsafe.Pointer(&eska[0])),
		(*C.uchar)(unsafe.Pointer(&pkb[0])))

	return ake_senda[:], tk[:], eska[:]
}

func KexAkeSharedB(ake_senda []byte, skb []byte, pka [PkLen]byte) ([]byte, [32]byte) {
	var ake_sendb [AkeSendB]byte
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

func XorKeys(x [SsLen]byte, y [SsLen]byte) [SsLen]byte {
	var out [SsLen]byte

	C.xor_keys(
		(*C.uchar)(unsafe.Pointer(&x[0])),
		(*C.uchar)(unsafe.Pointer(&y[0])),
		(*C.uchar)(unsafe.Pointer(&out[0])))

	return out
}

func GetRi() [CoinLen]byte {
	var coin [CoinLen]byte

	C.randombytes(
		(*C.uchar)(unsafe.Pointer(&coin[0])),
		CoinLen)

	return coin
}

func Sha3_512(x []byte) [64]byte {
	var out [64]byte

	C.hash_g_fn(
		(*C.uchar)(unsafe.Pointer(&out[0])),
		(*C.uchar)(unsafe.Pointer(&x[0])),
		(C.ulonglong)(len(x)))

	return out
}

func Commit_pke(pk [PkLen]byte, xi_i [SsLen + 4]byte, ri [CoinLen]byte) Commitment {
	var commitment Commitment

	C.commit(
		(*C.uchar)(unsafe.Pointer(&pk[0])),
		(*C.uchar)(unsafe.Pointer(&xi_i[0])),
		36,
		(*C.uchar)(unsafe.Pointer(&ri[0])),
		(*C.Commitment)(unsafe.Pointer(&commitment)))

	return commitment
}

func Mod(i int, j int) int {
	return int(C.mod(C.int(i), C.int(j)))
}
