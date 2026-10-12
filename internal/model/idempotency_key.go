package model

import "time"

type IdempotencyKey struct {
	Operation      string
	IdempotencyKey string
	RequestHash    []byte
	CardID         *string
	ResponseStatus *int16
	ResponseBody   []byte
	CreatedAt      time.Time
	ExpiresAt      time.Time
}
