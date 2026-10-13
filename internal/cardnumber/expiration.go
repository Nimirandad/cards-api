package cardnumber

import (
	"errors"
	"strconv"
	"time"
)

const CardValidityMonths = 60

var ErrInvalidExpirationDate = errors.New("expiration month must be MM and year must be YYYY")

func CalculateExpirationDate(issuedAt time.Time) time.Time {
	return time.Date(issuedAt.Year(), issuedAt.Month()+CardValidityMonths, 1, 0, 0, 0, 0, issuedAt.Location())
}

// ValidateExpirationDate requires a two-digit month and a four-digit year.
func ValidateExpirationDate(month, year string) error {
	if len(month) != 2 || len(year) != 4 {
		return ErrInvalidExpirationDate
	}

	for _, digit := range month + year {
		if digit < '0' || digit > '9' {
			return ErrInvalidExpirationDate
		}
	}

	intMonth, _ := strconv.Atoi(month)
	intYear, _ := strconv.Atoi(year)
	if intMonth < 1 || intMonth > 12 || intYear < 1 {
		return ErrInvalidExpirationDate
	}

	return nil
}

// IsCardExpired indicates if the expiration month and year have already passed.
// Card remains valid for the entire month of expiration.
func IsCardExpired(month, year string) bool {
	intMonth, _ := strconv.Atoi(month)
	intYear, _ := strconv.Atoi(year)

	now := time.Now().UTC()
	expirationDate := time.Date(intYear, time.Month(intMonth), 1, 0, 0, 0, 0, time.UTC)
	firstDayOfNextMonth := expirationDate.AddDate(0, 1, 0)

	return !now.Before(firstDayOfNextMonth)
}
