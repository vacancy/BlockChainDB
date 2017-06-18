package main

import (
    "fmt"
    "math/rand"

    "../hash"
)

func GetHashString(String string) string {
    return hash.GetHashString(String)
}

func GetHashBytes(String string) [32]byte {
    return hash.GetHashBytes(String)
}

func CheckHash(Hash string) bool {
    return hash.CheckHash(Hash)
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

func CheckNonce(nonce string) bool {
    if len(nonce) != 8 {
        return false
    }

    for _, c := range nonce {
        if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')) {
            return false
        }
    }
    return true
}
