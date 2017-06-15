package hash

import "crypto/sha256"
import "fmt"

func GetHashString(String string) string {
	return fmt.Sprintf("%x", GetHashBytes(String))
}

func GetHashBytes(String string) [32]byte {
	return sha256.Sum256([]byte(String))
}

func CheckHash(Hash string) bool {
	return Hash[0:5]=="00000"
}