package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Nimirandad/cards-api/internal/model"
	"github.com/Nimirandad/cards-api/internal/service"
)

const createCardOperation = "create_card"

type CardRepository struct {
	db             *sql.DB
	idempotencyTTL time.Duration
}

func NewCardRepository(db *sql.DB, idempotencyTTL time.Duration) *CardRepository {
	return &CardRepository{db: db, idempotencyTTL: idempotencyTTL}
}

// CreateCard creates a new card in the database while ensuring idempotency based on the provided key and request hash. 
// It returns the stored response body if the operation has already been performed with the same key and request hash.
func (r *CardRepository) CreateCard(ctx context.Context, key string, requestHash []byte, card model.Card, responseBody []byte) ([]byte, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create card transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	inserted, err := insertIdempotencyReservation(ctx, tx, key, requestHash, r.idempotencyTTL)
	if err != nil {
		return nil, err
	}

	if !inserted {
		storedHash, storedBody, expiresAt, err := r.getIdempotencyRecord(ctx, tx, key)
		if err != nil {
			return nil, err
		}

		if expiresAt.After(time.Now().UTC()) {
			if !bytes.Equal(storedHash, requestHash) {
				return nil, service.ErrIdempotencyKeyConflict
			}
			if len(storedBody) == 0 {
				return nil, errors.New("idempotency record has no stored response")
			}
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("commit idempotent replay: %w", err)
			}
			return storedBody, nil
		}

		deleteQuery := `
			DELETE FROM idempotency_keys WHERE operation = $1 AND idempotency_key = $2
		`

		if _, err := tx.ExecContext(ctx, deleteQuery, createCardOperation, key); err != nil {
			return nil, fmt.Errorf("delete expired idempotency record: %w", err)
		}

		inserted, err = insertIdempotencyReservation(ctx, tx, key, requestHash, r.idempotencyTTL)
		if err != nil {
			return nil, err
		}

		if !inserted {
			return nil, errors.New("could not reserve expired idempotency key")
		}
	}

	insertQuery := `
		INSERT INTO cards (id, holder_name, pan_hash, last4, status, issued_at, expiration_month, expiration_year) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	if _, err := tx.ExecContext(ctx, insertQuery,
		card.ID,
		card.HolderName,
		card.PANHash,
		card.Last4,
		card.Status,
		card.IssuedAt,
		card.ExpirationMonth,
		card.ExpirationYear,
	); err != nil {
		return nil, fmt.Errorf("insert card: %w", err)
	}

	updateQuery := `
		UPDATE idempotency_keys
		SET card_id = $1, response_status = $2, response_body = $3
		WHERE operation = $4 AND idempotency_key = $5
	`

	if _, err := tx.ExecContext(ctx, updateQuery, card.ID, 201, responseBody, createCardOperation, key); err != nil {
		return nil, fmt.Errorf("save idempotency response: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit create card transaction: %w", err)
	}

	return responseBody, nil
}

func insertIdempotencyReservation(ctx context.Context, tx *sql.Tx, key string, requestHash []byte, ttl time.Duration) (bool, error) {
	query := `
		INSERT INTO idempotency_keys (operation, idempotency_key, request_hash, expires_at) 
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (operation, idempotency_key) DO NOTHING
	`

	result, err := tx.ExecContext(ctx, query, createCardOperation, key, requestHash, time.Now().UTC().Add(ttl))
	if err != nil {
		return false, fmt.Errorf("reserve idempotency key: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read idempotency insert result: %w", err)
	}
	return rows == 1, nil
}

func (r *CardRepository) getIdempotencyRecord(ctx context.Context, tx *sql.Tx, key string) ([]byte, []byte, time.Time, error) {
	var requestHash, responseBody []byte
	var expiresAt time.Time
	query := `
		SELECT request_hash, response_body, expires_at
		FROM idempotency_keys
		WHERE operation = $1 AND idempotency_key = $2
		FOR UPDATE
	`

	err := tx.QueryRowContext(ctx, query, createCardOperation, key).Scan(&requestHash, &responseBody, &expiresAt)
	if err != nil {
		return nil, nil, time.Time{}, fmt.Errorf("load idempotency record: %w", err)
	}
	return requestHash, responseBody, expiresAt, nil
}

// GetCardByID retrieves a card from the database by its ID. It returns nil if no card is found.
func (r *CardRepository) GetCardByID(ctx context.Context, cardID string) (*model.Card, error) {
	query := `
		SELECT id, holder_name, last4, status, issued_at, expiration_month, expiration_year
		FROM cards
		WHERE id = $1
	`
	var card model.Card
	err := r.db.QueryRowContext(ctx, query, cardID).Scan(
		&card.ID,
		&card.HolderName,
		&card.Last4,
		&card.Status,
		&card.IssuedAt,
		&card.ExpirationMonth,
		&card.ExpirationYear,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load card by ID: %w", err)
	}
	return &card, nil
}

// ListCards retrieves a list of cards from the database with optional cursor-based pagination. 
// If cursorIssuedAt and cursorID are provided, it returns cards issued before the specified cursor.
func (r *CardRepository) ListCards(ctx context.Context, limit int, cursorIssuedAt *time.Time, cursorID string) ([]model.Card, error) {
	query := `
		SELECT id, holder_name, last4, status, issued_at, expiration_month, expiration_year
		FROM cards
		ORDER BY issued_at DESC, id DESC
		LIMIT $1
	`
	args := []any{limit}
	if cursorIssuedAt != nil {
		query = `
			SELECT id, holder_name, last4, status, issued_at, expiration_month, expiration_year
			FROM cards
			WHERE (issued_at, id) < ($1, $2::uuid)
			ORDER BY issued_at DESC, id DESC
			LIMIT $3
		`
		args = []any{*cursorIssuedAt, cursorID, limit}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	defer rows.Close()

	cards := make([]model.Card, 0, limit)
	for rows.Next() {
		var card model.Card
		if err := rows.Scan(&card.ID, &card.HolderName, &card.Last4, &card.Status, &card.IssuedAt, &card.ExpirationMonth, &card.ExpirationYear); err != nil {
			return nil, fmt.Errorf("scan listed card: %w", err)
		}
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate listed cards: %w", err)
	}
	return cards, nil
}

// SearchCardsByLast4 retrieves a list of cards from the database that match the provided last four digits of the PAN.
func (r *CardRepository) SearchCardsByLast4(ctx context.Context, last4Digits string) ([]model.Card, error) {
	query := `
		SELECT id, holder_name, last4, status, issued_at, expiration_month, expiration_year
		FROM cards
		WHERE last4 = $1
		ORDER BY issued_at DESC, id DESC
	`
	rows, err := r.db.QueryContext(ctx, query, last4Digits)
	if err != nil {
		return nil, fmt.Errorf("search cards by last4: %w", err)
	}
	defer rows.Close()

	cards := make([]model.Card, 0)
	for rows.Next() {
		var card model.Card
		if err := rows.Scan(&card.ID, &card.HolderName, &card.Last4, &card.Status, &card.IssuedAt, &card.ExpirationMonth, &card.ExpirationYear); err != nil {
			return nil, fmt.Errorf("scan card matching last4: %w", err)
		}
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cards matching last4: %w", err)
	}
	return cards, nil
}


// UpdateCardStatus updates the status of a card in the database by its ID.
func (r *CardRepository) UpdateCardStatus(ctx context.Context, cardID, status string) error {
	query := `
		UPDATE cards
		SET status = $1
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, query, status, cardID)
	if err != nil {
		return fmt.Errorf("update card status: %w", err)
	}
	return nil
}
