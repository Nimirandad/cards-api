package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Nimirandad/cards-api/internal/model"
	"github.com/Nimirandad/cards-api/internal/service"
)

const (
	InsertIdempotencyQuery = "INSERT INTO idempotency_keys (operation, idempotency_key, request_hash, expires_at)"
	InsertCardQuery        = "INSERT INTO cards (id, holder_name, pan_hash, last4, status, issued_at, expiration_month, expiration_year) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)"
	UpdateIdempotencyQuery = "UPDATE idempotency_keys SET card_id = $1, response_status = $2, response_body = $3"
	SelectIdempotencyQuery = "SELECT request_hash, response_body, expires_at FROM idempotency_keys WHERE operation = $1 AND idempotency_key = $2 FOR UPDATE"
	SelectCardByIDQuery    = "SELECT id, holder_name, last4, status, issued_at, expiration_month, expiration_year FROM cards WHERE id = $1"
	SelectCardListQuery    = "SELECT id, holder_name, last4, status, issued_at, expiration_month, expiration_year FROM cards ORDER BY issued_at DESC, id DESC LIMIT $1"
	SelectCardCursorQuery  = "SELECT id, holder_name, last4, status, issued_at, expiration_month, expiration_year FROM cards WHERE (issued_at, id) < ($1, $2::uuid) ORDER BY issued_at DESC, id DESC LIMIT $3"
	SelectCardByLast4Query = "SELECT id, holder_name, last4, status, issued_at, expiration_month, expiration_year FROM cards WHERE last4 = $1 ORDER BY issued_at DESC, id DESC"
	UpdateCardStatusQuery  = "UPDATE cards SET status = $1 WHERE id = $2"
)

func newMockCardRepository(t *testing.T) (*CardRepository, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet SQL expectations: %v", err)
		}
	})
	t.Cleanup(func() { _ = db.Close() })

	return NewCardRepository(db, 24*time.Hour), mock
}

func TestCardRepositoryCreateCard(t *testing.T) {
	t.Run("creates card and saves idempotent response in one transaction", func(t *testing.T) {
		repo, mock := newMockCardRepository(t)
		issuedAt := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
		card := model.Card{
			ID:              "2a8e78c0-f2e1-4c19-8376-249de6dbed37",
			HolderName:      "Ada Lovelace",
			PANHash:         []byte("pan-hash"),
			Last4:           "4242",
			Status:          service.CardStatusActive,
			IssuedAt:        issuedAt,
			ExpirationMonth: "10",
			ExpirationYear:  "2031",
		}
		requestHash := []byte("request-hash")
		responseBody := []byte(`{"id":"2a8e78c0-f2e1-4c19-8376-249de6dbed37"}`)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(InsertIdempotencyQuery)).WithArgs(createCardOperation, "request-1", requestHash, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(regexp.QuoteMeta(InsertCardQuery)).WithArgs(card.ID, card.HolderName, card.PANHash, card.Last4, card.Status, issuedAt, card.ExpirationMonth, card.ExpirationYear).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(UpdateIdempotencyQuery)).WithArgs(card.ID, 201, responseBody, createCardOperation, "request-1").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		got, err := repo.CreateCard(context.Background(), "request-1", requestHash, card, responseBody)
		if err != nil {
			t.Fatalf("CreateCard() error = %v", err)
		}
		if !bytes.Equal(got, responseBody) {
			t.Errorf("CreateCard() body = %s, want %s", got, responseBody)
		}
	})

	t.Run("replays a previous response for the same key and request", func(t *testing.T) {
		repo, mock := newMockCardRepository(t)
		now := time.Now().UTC()
		requestHash := []byte("same-request")
		responseBody := []byte(`{"id":"stored-card"}`)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(InsertIdempotencyQuery)).WithArgs(createCardOperation, "request-1", requestHash, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(regexp.QuoteMeta(SelectIdempotencyQuery)).WithArgs(createCardOperation, "request-1").WillReturnRows(sqlmock.NewRows([]string{"request_hash", "response_body", "expires_at"}).AddRow(requestHash, responseBody, now.Add(time.Hour)))
		mock.ExpectCommit()

		got, err := repo.CreateCard(context.Background(), "request-1", requestHash, model.Card{}, nil)
		if err != nil {
			t.Fatalf("CreateCard() error = %v", err)
		}
		if !bytes.Equal(got, responseBody) {
			t.Errorf("replayed body = %s, want %s", got, responseBody)
		}
	})

	t.Run("rejects reuse of a key with a different request", func(t *testing.T) {
		repo, mock := newMockCardRepository(t)
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(InsertIdempotencyQuery)).WithArgs(createCardOperation, "request-1", []byte("new-request"), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(regexp.QuoteMeta(SelectIdempotencyQuery)).WithArgs(createCardOperation, "request-1").WillReturnRows(sqlmock.NewRows([]string{"request_hash", "response_body", "expires_at"}).AddRow([]byte("original-request"), []byte(`{"id":"old"}`), time.Now().UTC().Add(time.Hour)))
		mock.ExpectRollback()

		_, err := repo.CreateCard(context.Background(), "request-1", []byte("new-request"), model.Card{}, nil)
		if !errors.Is(err, service.ErrIdempotencyKeyConflict) {
			t.Errorf("CreateCard() error = %v, want ErrIdempotencyKeyConflict", err)
		}
	})
}

func TestCardRepositoryGetCardByID(t *testing.T) {
	t.Run("returns card when found", func(t *testing.T) {
		repo, mock := newMockCardRepository(t)
		issuedAt := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
		mock.ExpectQuery(regexp.QuoteMeta(SelectCardByIDQuery)).WithArgs("2a8e78c0-f2e1-4c19-8376-249de6dbed37").WillReturnRows(sqlmock.NewRows([]string{"id", "holder_name", "last4", "status", "issued_at", "expiration_month", "expiration_year"}).
			AddRow("2a8e78c0-f2e1-4c19-8376-249de6dbed37", "Ada Lovelace", "4242", "active", issuedAt, "10", "2031"))

		card, err := repo.GetCardByID(context.Background(), "2a8e78c0-f2e1-4c19-8376-249de6dbed37")
		if err != nil {
			t.Fatalf("GetCardByID() error = %v", err)
		}
		if card == nil || card.HolderName != "Ada Lovelace" || card.Last4 != "4242" || !card.IssuedAt.Equal(issuedAt) {
			t.Errorf("GetCardByID() = %+v, want stored card", card)
		}
	})

	t.Run("returns nil when card does not exist", func(t *testing.T) {
		repo, mock := newMockCardRepository(t)
		mock.ExpectQuery(regexp.QuoteMeta(SelectCardByIDQuery)).WithArgs("missing-card").WillReturnError(sql.ErrNoRows)

		card, err := repo.GetCardByID(context.Background(), "missing-card")
		if err != nil || card != nil {
			t.Errorf("GetCardByID() = (%+v, %v), want (nil, nil)", card, err)
		}
	})
}

func TestCardRepositoryListCards(t *testing.T) {
	t.Run("lists first page", func(t *testing.T) {
		repo, mock := newMockCardRepository(t)
		issuedAt := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
		mock.ExpectQuery(regexp.QuoteMeta(SelectCardListQuery)).WithArgs(2).WillReturnRows(sqlmock.NewRows([]string{"id", "holder_name", "last4", "status", "issued_at", "expiration_month", "expiration_year"}).
			AddRow("card-1", "Ada", "4242", "active", issuedAt, "10", "2031").
			AddRow("card-2", "Grace", "1234", "active", issuedAt.Add(-time.Hour), "11", "2031"))

		cards, err := repo.ListCards(context.Background(), 2, nil, "")
		if err != nil {
			t.Fatalf("ListCards() error = %v", err)
		}
		if len(cards) != 2 || cards[0].ID != "card-1" || cards[1].ID != "card-2" {
			t.Errorf("ListCards() = %+v, want two cards in query order", cards)
		}
	})

	t.Run("uses the composite cursor for the next page", func(t *testing.T) {
		repo, mock := newMockCardRepository(t)
		cursorTime := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
		mock.ExpectQuery(regexp.QuoteMeta(SelectCardCursorQuery)).WithArgs(cursorTime, "2a8e78c0-f2e1-4c19-8376-249de6dbed37", 3).WillReturnRows(sqlmock.NewRows([]string{"id", "holder_name", "last4", "status", "issued_at", "expiration_month", "expiration_year"}))

		cards, err := repo.ListCards(context.Background(), 3, &cursorTime, "2a8e78c0-f2e1-4c19-8376-249de6dbed37")
		if err != nil {
			t.Fatalf("ListCards() error = %v", err)
		}
		if len(cards) != 0 {
			t.Errorf("ListCards() returned %d cards, want empty page", len(cards))
		}
	})
}

func TestCardRepositorySearchCardsByLast4(t *testing.T) {
	repo, mock := newMockCardRepository(t)
	issuedAt := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(SelectCardByLast4Query)).WithArgs("4242").WillReturnRows(sqlmock.NewRows([]string{"id", "holder_name", "last4", "status", "issued_at", "expiration_month", "expiration_year"}).
		AddRow("card-1", "Ada", "4242", "active", issuedAt, "10", "2031").
		AddRow("card-2", "Grace", "4242", "expired", issuedAt.Add(-time.Hour), "11", "2031"))

	cards, err := repo.SearchCardsByLast4(context.Background(), "4242")
	if err != nil {
		t.Fatalf("SearchCardsByLast4() error = %v", err)
	}
	if len(cards) != 2 || cards[0].Last4 != "4242" || cards[1].Last4 != "4242" {
		t.Errorf("SearchCardsByLast4() = %+v, want both matching cards", cards)
	}
}

func TestCardRepositoryUpdateCardStatus(t *testing.T) {
	repo, mock := newMockCardRepository(t)
	mock.ExpectExec(regexp.QuoteMeta(UpdateCardStatusQuery)).WithArgs(service.CardStatusExpired, "card-1").WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.UpdateCardStatus(context.Background(), "card-1", service.CardStatusExpired); err != nil {
		t.Fatalf("UpdateCardStatus() error = %v", err)
	}
}
