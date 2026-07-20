package middleware

import "crypto/subtle"

// subtleCompare performs a constant-time string comparison to avoid timing leaks.
func subtleCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
