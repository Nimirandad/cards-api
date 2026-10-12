// Package cardnumber contiene generadores de identificadores numéricos de prueba.
package cardnumber

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const (
	VisaPrefix   = "4"
	RandomDigits = 14
)

// GenerateCardNumber generates a random valid card number with a Visa prefix and a valid Luhn check digit.
func GenerateCardNumber() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(100_000_000_000_000))
	if err != nil {
		return "", fmt.Errorf("generar número aleatorio: %w", err)
	}
	body := fmt.Sprintf("%s%0*d", VisaPrefix, RandomDigits, n)
	return AppendLuhnCheckDigit(body)
}
