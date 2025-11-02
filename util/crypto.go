package util

import (
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
