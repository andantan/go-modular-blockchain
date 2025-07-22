package random

import "crypto/rand"

func GenerateRandomBytes(size int) []byte {
	token := make([]byte, size)
	_, _ = rand.Read(token)

	return token
}
