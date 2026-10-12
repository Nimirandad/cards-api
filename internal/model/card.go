package model

import "time"

type Card struct {
	ID              string    `json:"id"`
	HolderName      string    `json:"holder_name"`
	PANHash         []byte    `json:"pan_hash"`
	Last4           string    `json:"last4"`
	Status          string    `json:"status"`
	IssuedAt        time.Time `json:"issued_at"`
	ExpirationMonth string    `json:"expiration_month"`
	ExpirationYear  string    `json:"expiration_year"`
}
