package utils

import (
	"crypto/rand"
	"time"
)

const idAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateEntityID builds a human-scannable ID in the prefix + YYMMDD + 6
// random uppercase alnum chars shape used across the campaign module
// (Order/Audience/Creative/Transaction — see cart-transaction.md), e.g.
// GenerateEntityID("") -> "260805Y4GK6W", GenerateEntityID("TRX-") ->
// "TRX-260805EEDDJ6".
func GenerateEntityID(prefix string) string {
	buf := make([]byte, 6)
	rand.Read(buf) // crypto/rand.Read on the default reader never errors
	for i, b := range buf {
		buf[i] = idAlphabet[int(b)%len(idAlphabet)]
	}
	return prefix + time.Now().Format("060102") + string(buf)
}
