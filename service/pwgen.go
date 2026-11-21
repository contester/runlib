package service

import (
	"fmt"

	"lukechampine.com/blake3"
)

const (
	pwLower    = "abcdefghijklmnopqrstuvwxyz"
	pwUpper    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	pwDigit    = "0123456789"
	pwAlphabet = pwLower + pwUpper + pwDigit
	pwLen      = 12
)

// generatePasswords derives deterministic 12-character ASCII alphanumeric
// passwords for tester0..N from BLAKE3-256("contester-tester:<hostname>:<index>").
func generatePasswords(hostname string, count int) []string {
	if count <= 0 {
		return nil
	}
	result := make([]string, count)
	for i := range count {
		sum := blake3.Sum256(fmt.Appendf(nil, "contester-tester:%s:%d", hostname, i))
		var pw [pwLen]byte
		pw[0] = pwLower[int(sum[0])%len(pwLower)]
		pw[1] = pwUpper[int(sum[1])%len(pwUpper)]
		pw[2] = pwDigit[int(sum[2])%len(pwDigit)]
		for j := 3; j < pwLen; j++ {
			pw[j] = pwAlphabet[int(sum[j])%len(pwAlphabet)]
		}
		result[i] = string(pw[:])
	}
	return result
}
