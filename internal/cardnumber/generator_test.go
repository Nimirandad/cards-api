package cardnumber

import (
	"testing"
)

func TestGenerateCardNumber(t *testing.T) {
	t.Run("GenerateCardNumber returns a 16-digit card starting with 4", func(t *testing.T) {
		card, err := GenerateCardNumber()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(card) != 16 {
			t.Fatalf("expected card length 16, got %d", len(card))
		}
		if card[0] != '4' {
			t.Fatalf("expected card to start with 4, got %s", card)
		}
	})
}
