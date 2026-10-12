package cardnumber

import "testing"

func TestIsCardExpired(t *testing.T) {
	t.Run("expired card", func(t *testing.T) {
		if !IsCardExpired("01", "2000") {
			t.Error("expected card to be expired")
		}
	})
	t.Run("valid card", func(t *testing.T) {
		if IsCardExpired("12", "2099") {
			t.Error("expected card to be valid")
		}
	})
}