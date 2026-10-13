package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Nimirandad/cards-api/internal/service"
)

type CardUseCase interface {
	Create(context.Context, string, string, string, string, string, bool) (service.CreateCardResult, error)
	CardByID(context.Context, string) (service.CreateCardResult, error)
	List(context.Context, int, string) (service.CardListResult, error)
	SearchByLast4(context.Context, string) ([]service.CreateCardResult, error)
}

type CardHandler struct {
	service CardUseCase
}

type CreateCardRequest struct {
	HolderName      string `json:"holder_name"`
	PAN             string `json:"pan,omitempty"`
	ExpirationMonth string `json:"expiration_month,omitempty"`
	ExpirationYear  string `json:"expiration_year,omitempty"`
	IsManual        bool   `json:"is_manual,omitempty"`
}

type CardResponse struct {
	ID             string `json:"id"`
	HolderName     string `json:"holder_name"`
	MaskedPAN      string `json:"masked_pan"`
	Status         string `json:"status"`
	ExpirationDate string `json:"expiration_date"`
}

type CardListResponse struct {
	Data     []CardResponse  `json:"data"`
	PageInfo CardPageInfoDTO `json:"page_info"`
}

type CardSearchResponse struct {
	Cards []CardResponse `json:"cards"`
}

type CardPageInfoDTO struct {
	HasNextPage bool    `json:"has_next_page"`
	EndCursor   *string `json:"end_cursor"`
}

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewCardHandler(cardService CardUseCase) *CardHandler {
	return &CardHandler{service: cardService}
}

// CreateCard handles the POST /cards endpoint, allowing clients to create a new card with an idempotency key.
func (h *CardHandler) CreateCard(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" || len(idempotencyKey) > 255 {
		writeError(w, http.StatusBadRequest, "invalid_idempotency_key", "missing or invalid Idempotency-Key header")
		return
	}

	var req CreateCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_body", "invalid request body")
		return
	}

	result, err := h.service.Create(r.Context(), idempotencyKey, req.HolderName, req.PAN, req.ExpirationMonth, req.ExpirationYear, req.IsManual)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPAN):
			writeError(w, http.StatusBadRequest, "invalid_pan", err.Error())
		case errors.Is(err, service.ErrInvalidExpiration):
			writeError(w, http.StatusBadRequest, "invalid_expiration", err.Error())
		case errors.Is(err, service.ErrExpiredCard):
			writeError(w, http.StatusBadRequest, "expired_card", err.Error())
		case errors.Is(err, service.ErrManualCardDataRequired):
			writeError(w, http.StatusBadRequest, "invalid_manual_card_request", err.Error())
		case errors.Is(err, service.ErrIdempotencyKeyConflict):
			writeError(w, http.StatusConflict, "idempotency_key_conflict", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "create_card_failed", "failed to create card")
		}
		return
	}

	resp := CardResponse{
		ID:             result.Card.ID,
		HolderName:     result.Card.HolderName,
		MaskedPAN:      result.MaskedPAN,
		Status:         result.Card.Status,
		ExpirationDate: fmt.Sprintf("%s-%s", result.Card.ExpirationYear, result.Card.ExpirationMonth),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetCard handles the GET /cards/{id} endpoint, allowing clients to retrieve a card by its ID.
func (h *CardHandler) GetCard(w http.ResponseWriter, r *http.Request) {
	cardID := r.PathValue("id")
	if cardID == "" {
		writeError(w, http.StatusBadRequest, "missing_card_id", "missing card ID")
		return
	}

	res, err := h.service.CardByID(r.Context(), cardID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCardNotFound):
			writeError(w, http.StatusNotFound, "card_not_found", "card not found")
		default:
			writeError(w, http.StatusInternalServerError, "get_card_failed", "failed to get card")
		}
		return
	}

	resp := CardResponse{
		ID:             res.Card.ID,
		HolderName:     res.Card.HolderName,
		MaskedPAN:      res.MaskedPAN,
		Status:         res.Card.Status,
		ExpirationDate: fmt.Sprintf("%s-%s", res.Card.ExpirationYear, res.Card.ExpirationMonth),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// ListCards handles the GET /cards endpoint, allowing clients to retrieve a cursor-based paginated list of cards.
func (h *CardHandler) ListCards(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limitValue := query.Get("limit")
	limit := service.DefaultCardPageSize
	if limitValue != "" {
		parsedLimit, err := strconv.Atoi(limitValue)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_limit", service.ErrInvalidLimit.Error())
			return
		}
		limit = parsedLimit
	}

	page, err := h.service.List(r.Context(), limit, query.Get("cursor"))
	if err != nil {
		if errors.Is(err, service.ErrInvalidLimit) {
			writeError(w, http.StatusBadRequest, "invalid_limit", err.Error())
		} else if errors.Is(err, service.ErrInvalidCursor) {
			writeError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "list_cards_failed", "failed to list cards")
		}
		return
	}

	resp := CardListResponse{
		Data: make([]CardResponse, len(page.Items)),
		PageInfo: CardPageInfoDTO{
			HasNextPage: page.HasNextPage,
		},
	}
	for i, item := range page.Items {
		resp.Data[i] = CardResponse{
			ID:             item.Card.ID,
			HolderName:     item.Card.HolderName,
			MaskedPAN:      item.MaskedPAN,
			Status:         item.Card.Status,
			ExpirationDate: fmt.Sprintf("%s-%s", item.Card.ExpirationYear, item.Card.ExpirationMonth),
		}
	}
	if page.EndCursor != "" {
		resp.PageInfo.EndCursor = &page.EndCursor
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// SearchCards handles the GET /cards:search endpoint, allowing clients to search for cards by the last four digits of the PAN.
func (h *CardHandler) SearchCards(w http.ResponseWriter, r *http.Request) {
	last4Digits := r.URL.Query().Get("last4")
	if last4Digits == "" || len(last4Digits) != 4 {
		writeError(w, http.StatusBadRequest, "invalid_last4", "last4 must contain exactly four ASCII digits")
		return
	}

	res, err := h.service.SearchByLast4(r.Context(), last4Digits)
	if err != nil {
		if errors.Is(err, service.ErrInvalidLast4) {
			writeError(w, http.StatusBadRequest, "invalid_last4", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "search_cards_failed", "failed to search cards")
		return
	}

	if len(res) == 0 {
		writeError(w, http.StatusNotFound, "card_not_found", "no cards found with the provided last4")
		return
	}

	resp := CardSearchResponse{Cards: make([]CardResponse, len(res))}
	for i, r := range res {
		resp.Cards[i] = CardResponse{
			ID:             r.Card.ID,
			HolderName:     r.Card.HolderName,
			MaskedPAN:      r.MaskedPAN,
			Status:         r.Card.Status,
			ExpirationDate: fmt.Sprintf("%s-%s", r.Card.ExpirationYear, r.Card.ExpirationMonth),
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error: errorDetail{
			Code:    code,
			Message: message,
		},
	})
}
