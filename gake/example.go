package gake

import (
	"fmt"
)

func Example() [64]byte {
	numParties := 3
	kem_a := GetKemKeyPair()
	kem_b := GetKemKeyPair()
	kem_c := GetKemKeyPair()
	var parties = make([]Party, numParties)
	parties[0].Pk = kem_a.Pk
	parties[0].Sk = kem_a.Sk
	parties[1].Pk = kem_b.Pk
	parties[1].Sk = kem_b.Sk
	parties[2].Pk = kem_c.Pk
	parties[2].Sk = kem_c.Sk
	for i := 0; i < numParties; i++ {
		parties[i].Xs = make([][32]byte, numParties)
		parties[i].Coins = make([][44]byte, numParties)
		parties[i].Commitments = make([]Commitment, numParties)
		parties[i].MasterKey = make([][32]byte, numParties)
	}
	pids := make([][20]byte, numParties)
	// Compute left right keys
	for i := 0; i < numParties; i++ {
		var right = (i + 1) % numParties
		var left = (i - 1 + numParties) % numParties
		ake_senda, tk, eska := KexAkeInitA(parties[right].Pk)
		ake_sendb, kb := KexAkeSharedB(ake_senda, parties[right].Sk[:], parties[i].Pk)
		ka := KexAkeSharedA(ake_sendb, tk, eska, parties[i].Sk[:])
		copy(parties[i].KeyRight[:], ka[:])
		copy(parties[right].KeyLeft[:], kb[:])
		ake_senda2, tk2, eska2 := KexAkeInitA(parties[left].Pk)
		ake_sendb2, kb2 := KexAkeSharedB(ake_senda2, parties[left].Sk[:], parties[i].Pk)
		ka2 := KexAkeSharedA(ake_sendb2, tk2, eska2, parties[i].Sk[:])
		copy(parties[i].KeyLeft[:], ka2[:])
		copy(parties[left].KeyRight[:], kb2[:])
	}
	// Compute Xs and commitments
	for i := 0; i < numParties; i++ {
		xi, ri, commitment := ComputeXsCommitment(
			i,
			parties[i].KeyRight,
			parties[i].KeyLeft,
			parties[i].Pk)
		for j := 0; j < numParties; j++ {
			copy(parties[j].Xs[i][:], xi[:])
			copy(parties[j].Coins[i][:], ri[:])
			copy(parties[j].Commitments[i].CipherTextKem[:], commitment.CipherTextKem[:])
			copy(parties[j].Commitments[i].CipherTextDem[:], commitment.CipherTextDem[:])
			copy(parties[j].Commitments[i].Tag[:], commitment.Tag[:])
		}
	}
	// Computer Master keys
	for i := 0; i < numParties; i++ {
		masterkey := ComputeMasterKey(numParties, i, parties[i].KeyLeft, parties[i].Xs)
		copy(parties[i].MasterKey, masterkey)
	}
	for i := 0; i < numParties; i++ {
		//PrintParty(parties[i])
	}
	sksid := make([][64]byte, numParties)
	// Compute sk_sid
	for i := 0; i < numParties; i++ {
		sksid[i] = ComputeSkSid(numParties, parties[i].MasterKey, pids)
	}
	for i := 0; i < len(sksid); i++ {
		fmt.Printf("sksid%d: %02x\n\n", i, sksid[i])
	}
	return sksid[0]
}
