//go:build ignore

// Alat bantu: cetak hash bcrypt untuk seed lokal.
// Jalankan: go run scripts/hashpw.go <password>
package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "pakai: go run scripts/hashpw.go <password>")
		os.Exit(2)
	}
	pw := os.Args[1]
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(h))
}
