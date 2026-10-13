package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Nimirandad/cards-api/internal/cardnumber"
	"github.com/Nimirandad/cards-api/internal/model"
	"github.com/google/uuid"
)

const (
	CardStatusActive    = "active"
	CardStatusExpired   = "expired"
	DefaultCardPageSize = 20
	MaxCardPageSize     = 100
)

var (
	ErrIdempotencyKeyConflict = errors.New("idempotency key already used with different data")
	ErrInvalidPAN             = errors.New("invalid PAN")
	ErrInvalidExpiration      = cardnumber.ErrInvalidExpirationDate
	ErrExpiredCard            = errors.New("card is expired")
	ErrManualCardDataRequired = errors.New("manual card creation requires PAN, expiration month and year")
	ErrCardNotFound           = errors.New("card not found")
	ErrInvalidLimit           = errors.New("limit must be between 1 and 100")
	ErrInvalidCursor          = errors.New("invalid pagination cursor")
	ErrInvalidLast4           = cardnumber.ErrInvalidLast4
)

// CardRepository define la persistencia de tarjetas y de las respuestas idempotentes.
type CardRepository interface {
	CreateCard(ctx context.Context, key string, requestHash []byte, card model.Card, responseBody []byte) ([]byte, error)
	GetCardByID(ctx context.Context, cardID string) (*model.Card, error)
	UpdateCardStatus(ctx context.Context, cardID, status string) error
	ListCards(ctx context.Context, limit int, cursorIssuedAt *time.Time, cursorID string) ([]model.Card, error)
	SearchCardsByLast4(ctx context.Context, last4Digits string) ([]model.Card, error)
}

type CardService struct {
	repository CardRepository
	panSalt    []byte
}

// CreateCardResult contiene el modelo persistido y los datos seguros para responder.
type CreateCardResult struct {
	Card      model.Card `json:"card"`
	MaskedPAN string     `json:"masked_pan"`
}

type CardListResult struct {
	Items       []CreateCardResult
	HasNextPage bool
	EndCursor   string
}

type cardListCursor struct {
	IssuedAt time.Time `json:"issued_at"`
	ID       string    `json:"id"`
}

func NewCardService(repository CardRepository, panSalt string) *CardService {
	return &CardService{repository: repository, panSalt: []byte(panSalt)}
}

// Create creates a new card with the provided holder name and PAN, ensuring idempotency based on the provided key.
func (s *CardService) Create(ctx context.Context, idempotencyKey, holderName, pan, expirationMonth, expirationYear string, isManual bool) (CreateCardResult, error) {
	if err := ctx.Err(); err != nil {
		return CreateCardResult{}, err
	}
	if idempotencyKey == "" {
		return CreateCardResult{}, errors.New("idempotency key required")
	}

	requestPAN := pan
	if isManual {
		if pan == "" || expirationMonth == "" || expirationYear == "" {
			return CreateCardResult{}, ErrManualCardDataRequired
		}

		if !cardnumber.ValidateLuhn(pan) {
			return CreateCardResult{}, ErrInvalidPAN
		}

		if err := cardnumber.ValidateExpirationDate(expirationMonth, expirationYear); err != nil {
			return CreateCardResult{}, ErrInvalidExpiration
		}

		if cardnumber.IsCardExpired(expirationMonth, expirationYear) {
			return CreateCardResult{}, ErrExpiredCard
		}
	} else {
		var err error
		pan, err = cardnumber.GenerateCardNumber()
		if err != nil {
			return CreateCardResult{}, err
		}
	}

	requestHash, err := s.idempotencyRequestHash(holderName, requestPAN, expirationMonth, expirationYear, isManual)
	if err != nil {
		return CreateCardResult{}, err
	}

	cardID, err := uuid.NewRandom()
	if err != nil {
		return CreateCardResult{}, err
	}

	last4Digits := pan[len(pan)-4:]
	maskedPAN, err := cardnumber.MaskPAN(last4Digits)
	if err != nil {
		return CreateCardResult{}, err
	}

	issuedAt := time.Now().UTC()
	expiration := cardnumber.CalculateExpirationDate(issuedAt)
	expirationMonthToStore := expiration.Format("01")
	expirationYearToStore := expiration.Format("2006")
	if isManual {
		expirationMonthToStore = expirationMonth
		expirationYearToStore = expirationYear
	}
	panMAC := hmac.New(sha256.New, s.panSalt)
	_, _ = panMAC.Write([]byte(pan))
	card := model.Card{
		ID:              cardID.String(),
		HolderName:      holderName,
		PANHash:         panMAC.Sum(nil),
		Last4:           last4Digits,
		Status:          CardStatusActive,
		IssuedAt:        issuedAt,
		ExpirationMonth: expirationMonthToStore,
		ExpirationYear:  expirationYearToStore,
	}
	result := CreateCardResult{Card: card, MaskedPAN: maskedPAN}
	responseBody, err := json.Marshal(result)
	if err != nil {
		return CreateCardResult{}, err
	}

	storedBody, err := s.repository.CreateCard(ctx, idempotencyKey, requestHash, card, responseBody)
	if err != nil {
		return CreateCardResult{}, err
	}

	if err := json.Unmarshal(storedBody, &result); err != nil {
		return CreateCardResult{}, err
	}

	return result, nil
}

func (s *CardService) idempotencyRequestHash(holderName, pan, expirationMonth, expirationYear string, manual bool) ([]byte, error) {
	type RequestHash struct {
		HolderName      string `json:"holder_name"`
		PAN             string `json:"pan"`
		ExpirationMonth string `json:"expiration_month,omitempty"`
		ExpirationYear  string `json:"expiration_year,omitempty"`
		Manual          bool   `json:"manual,omitempty"`
	}

	request, err := json.Marshal(RequestHash{
		HolderName:      holderName,
		PAN:             pan,
		ExpirationMonth: expirationMonth,
		ExpirationYear:  expirationYear,
		Manual:          manual,
	})
	if err != nil {
		return nil, err
	}

	keyMAC := hmac.New(sha256.New, s.panSalt)
	_, _ = keyMAC.Write([]byte("cards-api:create-card:idempotency:v1"))
	requestKey := keyMAC.Sum(nil)

	requestMAC := hmac.New(sha256.New, requestKey)
	_, _ = requestMAC.Write(request)
	return requestMAC.Sum(nil), nil
}

// CardByID retrieves a card by its ID, returning an error if the card is not found or if the context is canceled.
func (s *CardService) CardByID(ctx context.Context, cardID string) (CreateCardResult, error) {
	if err := ctx.Err(); err != nil {
		return CreateCardResult{}, err
	}
	storedCard, err := s.repository.GetCardByID(ctx, cardID)
	if err != nil {
		return CreateCardResult{}, err
	}
	if storedCard == nil {
		return CreateCardResult{}, ErrCardNotFound
	}

	if err := s.refreshExpirationStatus(ctx, storedCard); err != nil {
		return CreateCardResult{}, err
	}

	maskedPAN, err := cardnumber.MaskPAN(storedCard.Last4)
	if err != nil {
		return CreateCardResult{}, err
	}

	result := CreateCardResult{
		Card:      *storedCard,
		MaskedPAN: maskedPAN,
	}
	return result, nil
}

// List retrieves a paginated list of cards, applying cursor-based pagination and refreshing the expiration status of each card.
func (s *CardService) List(ctx context.Context, limit int, rawCursor string) (CardListResult, error) {
	if err := ctx.Err(); err != nil {
		return CardListResult{}, err
	}
	if limit < 1 || limit > MaxCardPageSize {
		return CardListResult{}, ErrInvalidLimit
	}

	var cursorIssuedAt *time.Time
	cursorID := ""
	if rawCursor != "" {
		cursor, err := decodeCardListCursor(rawCursor)
		if err != nil {
			return CardListResult{}, err
		}
		cursorIssuedAt = &cursor.IssuedAt
		cursorID = cursor.ID
	}

	cards, err := s.repository.ListCards(ctx, limit+1, cursorIssuedAt, cursorID)
	if err != nil {
		return CardListResult{}, err
	}

	hasMore := len(cards) > limit
	if hasMore {
		cards = cards[:limit]
	}

	result := CardListResult{Items: make([]CreateCardResult, 0, len(cards)), HasNextPage: hasMore}
	for i := range cards {
		if err := s.refreshExpirationStatus(ctx, &cards[i]); err != nil {
			return CardListResult{}, err
		}
		maskedPAN, err := cardnumber.MaskPAN(cards[i].Last4)
		if err != nil {
			return CardListResult{}, err
		}
		result.Items = append(result.Items, CreateCardResult{Card: cards[i], MaskedPAN: maskedPAN})
	}

	if hasMore {
		last := cards[len(cards)-1]
		result.EndCursor, err = encodeCardListCursor(cardListCursor{IssuedAt: last.IssuedAt, ID: last.ID})
		if err != nil {
			return CardListResult{}, err
		}
	}

	return result, nil
}

func (s *CardService) refreshExpirationStatus(ctx context.Context, card *model.Card) error {
	if card.Status != CardStatusActive || !cardnumber.IsCardExpired(card.ExpirationMonth, card.ExpirationYear) {
		return nil
	}

	if err := s.repository.UpdateCardStatus(ctx, card.ID, CardStatusExpired); err != nil {
		return err
	}

	card.Status = CardStatusExpired
	return nil
}

func encodeCardListCursor(cursor cardListCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("encode pagination cursor: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeCardListCursor(raw string) (cardListCursor, error) {
	if len(raw) > 512 {
		return cardListCursor{}, ErrInvalidCursor
	}

	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cardListCursor{}, ErrInvalidCursor
	}

	var cursor cardListCursor
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.IssuedAt.IsZero() || uuid.Validate(cursor.ID) != nil {
		return cardListCursor{}, ErrInvalidCursor
	}

	return cursor, nil
}

// SearchByLast4 retrieves a list of cards that match the provided last four digits of the PAN,
// ensuring that the input is valid and refreshing the expiration status of each card.
func (s *CardService) SearchByLast4(ctx context.Context, last4Digits string) ([]CreateCardResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := cardnumber.MaskPAN(last4Digits); err != nil {
		return nil, ErrInvalidLast4
	}

	cards, err := s.repository.SearchCardsByLast4(ctx, last4Digits)
	if err != nil {
		return nil, err
	}

	results := make([]CreateCardResult, 0, len(cards))
	for i := range cards {
		if err := s.refreshExpirationStatus(ctx, &cards[i]); err != nil {
			return nil, err
		}
		maskedPAN, err := cardnumber.MaskPAN(cards[i].Last4)
		if err != nil {
			return nil, err
		}
		results = append(results, CreateCardResult{Card: cards[i], MaskedPAN: maskedPAN})
	}
	return results, nil
}
