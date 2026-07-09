package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	hash, err := bcrypt.GenerateFromPassword([]byte("<REDACTED>"), 12)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(string(hash))
}
