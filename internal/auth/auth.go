package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

const storedPasswordHash = "fa4a333953c12a3bb2daf608ad397dcd7005821fb5ed53df95cb390234b32970"

func IsValid(password string) bool {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:]) == storedPasswordHash
}
