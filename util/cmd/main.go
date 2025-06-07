package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"pqgch/gake"
)

func main() {
	count := flag.Int("c", 1, "number of keypairs to generate")
	flag.Parse()

	keyPairs := make([]gake.KemKeyPair, *count)

	for i := range *count {
		keyPairs[i] = gake.GetKemKeyPair()
	}

	fmt.Printf("\nprinting public keys 0..%d\n\n", *count-1)
	fmt.Println("\"publicKeys\": [")
	for i := range *count {
		fmt.Printf("\"%s\"", base64.StdEncoding.EncodeToString(keyPairs[i].Pk[:]))
		if i < *count-1 {
			fmt.Println(",")
		}
	}
	fmt.Println("\n]")

	fmt.Printf("\nprinting secret keys 0..%d\n", *count-1)
	for i := range *count {
		fmt.Printf("\n\"secretKey\": \"%s\"\n", base64.StdEncoding.EncodeToString(keyPairs[i].Sk[:]))
	}
}
