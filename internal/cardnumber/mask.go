package cardnumber

import (
	"errors"
	"strings"
)

var ErrInvalidLast4 = errors.New("last4 must contain exactly four ASCII digits")

// MaskPAN masks a credit card number, leaving only the last four digits visible.
// For example, sending the last four digits "4242" converts to "************4242" by adding asterisks at the beginning.
func MaskPAN(last4 string) (string, error) {
	if len(last4) != 4 {
		return "", ErrInvalidLast4
	}
	for i := 0; i < len(last4); i++ {
		if last4[i] < '0' || last4[i] > '9' {
			return "", ErrInvalidLast4
		}
	}
	return strings.Repeat("*", maxPANLength-4) + last4, nil
}
