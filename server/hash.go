package main

import (
    "crypto/sha256"
    "fmt"
    "math/rand"
)

func GetHashString(String string) string {
	return fmt.Sprintf("%x", GetHashBytes(String))
}

func GetHashBytes(String string) [32]byte {
	return sha256.Sum256([]byte(String))
}

func CheckHash(Hash string) bool {
	return Hash[0:5]=="00000"
}

func UUID128bit() string {
    // Returns a 128bit hex string, RFC4122-compliant UUIDv4
    u := make([]byte,16)
    _, _ = rand.Read(u)
    // this make sure that the 13th character is "4"
    u[6] = (u[6] | 0x40) & 0x4F
    // this make sure that the 17th is "8", "9", "a", or "b"
    u[8] = (u[8] | 0x80) & 0xBF
    return fmt.Sprintf("%x", u)
}
