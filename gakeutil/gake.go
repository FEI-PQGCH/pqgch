package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"pqgch/gake"
)

func main() {
	programFlag := flag.String("p", "", "program to run")
	countFlag := flag.Int("c", 1, "number of keypairs to generate")

	flag.Parse()

	if *programFlag == "" {
		fmt.Println("please provide a program to run using the -p flag.")
		return
	}

	switch *programFlag {
	case "gen":
		keyGen(countFlag)
	case "test":
		gake.Example()
	}
}

func keyGen(countFlag *int) {
	keyPairs := make([]gake.KemKeyPair, *countFlag)

	for i := 0; i < *countFlag; i++ {
		keyPairs[i] = gake.GetKemKeyPair()
	}

	fmt.Printf("\nprinting public keys 0..%d\n\n", *countFlag-1)
	fmt.Println("\"publicKeys\": [")
	for i := 0; i < *countFlag; i++ {
		fmt.Printf("\"%s\"", base64.StdEncoding.EncodeToString(keyPairs[i].Pk[:]))
		if i < *countFlag-1 {
			fmt.Println(",")
		}
	}
	fmt.Println("\n]")

	fmt.Printf("\nprinting secret keys 0..%d\n", *countFlag-1)
	for i := 0; i < *countFlag; i++ {
		fmt.Printf("\n\"secretKey\": \"%s\"\n", base64.StdEncoding.EncodeToString(keyPairs[i].Sk[:]))
	}

}
