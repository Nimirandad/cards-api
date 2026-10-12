package router

import (
	"net/http"

	"github.com/Nimirandad/cards-api/internal/handler"
)

func New(cards *handler.CardHandler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /cards", cards.CreateCard)
	mux.HandleFunc("GET /cards/{id}", cards.GetCard)
	mux.HandleFunc("GET /cards", cards.ListCards)
	mux.HandleFunc("GET /cards:search", cards.SearchCards)
	return mux
}
