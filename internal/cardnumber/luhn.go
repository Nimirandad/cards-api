package cardnumber

import (
	"errors"
)

const (
	maxPANLength = 16
)

var ErrInvalidNumber = errors.New("the number must contain only ASCII digits")

// AppendLuhnCheckDigit calculates and appends the Luhn check digit to a sequence of digits.
func AppendLuhnCheckDigit(number string) (string, error) {
	sum, err := luhnSum(number, true)
	if err != nil {
		return "", err
	}
	checkDigit := (10 - sum%10) % 10
	return number + string(rune('0'+checkDigit)), nil
}

// ValidateLuhn returns true if number has 16 ASCII digits and passes the Luhn algorithm.
func ValidateLuhn(number string) bool {
	if len(number) != maxPANLength {
		return false
	}
	for i := 0; i < len(number); i++ {
		if number[i] < '0' || number[i] > '9' {
			return false
		}
	}

	sum, err := luhnSum(number, false)
	if err != nil {
		return false
	}
	return sum%10 == 0
}

// luhnSum validates the digits and calculates the Luhn sum. isSecondDigit indicates
// if the rightmost digit should be doubled.
func luhnSum(number string, isSecondDigit bool) (int, error) {
	if number == "" {
		return 0, ErrInvalidNumber
	}

	sum := 0
	for i := len(number) - 1; i >= 0; i-- {
		digit := int(number[i] - '0')
		if isSecondDigit {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		isSecondDigit = !isSecondDigit
	}
	return sum, nil
}
