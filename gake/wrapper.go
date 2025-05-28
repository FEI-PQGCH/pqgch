package gake

/*
#cgo CFLAGS: -DKYBER_K=4 -I./kyber-gake/ref -Wall  -Wextra -Wpedantic -Werror -Wshadow -Wpointer-arith -O3 -fwrapv
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
	PkLen    = 1568
	SkLen    = 3168
	CtKemLen = 1568
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

func KexAkeSharedB(ake_senda []byte, skb []byte, pka [1568]byte) ([]byte, [32]byte) {
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

func XorKeys(x [32]byte, y [32]byte) [32]byte {
	var out [32]byte

	C.xor_keys(
		(*C.uchar)(unsafe.Pointer(&x[0])),
		(*C.uchar)(unsafe.Pointer(&y[0])),
		(*C.uchar)(unsafe.Pointer(&out[0])))

	return out
}

func GetRi() [44]byte {
	var coin [44]byte

	C.randombytes(
		(*C.uchar)(unsafe.Pointer(&coin[0])),
		44)

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

func Commit_pke(pk [PkLen]byte, xi_i [36]byte, ri [44]byte) Commitment {
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
