package cardnumber

import (
	"strconv"
	"time"
)

const CardValidityMonths = 60

func CalculateExpirationDate(issuedAt time.Time) time.Time {
	return time.Date(issuedAt.Year(), issuedAt.Month()+CardValidityMonths, 1, 0, 0, 0, 0, issuedAt.Location())
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
