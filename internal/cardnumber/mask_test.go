package cardnumber

import "testing"

func TestMaskPAN(t *testing.T) {
	t.Run("mask PAN correctly", func(t *testing.T) {
		last4 := "5678"
		expected := "************5678"
		masked, err := MaskPAN(last4)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if masked != expected {
			t.Errorf("expected %s, got %s", expected, masked)
		}
	})

	t.Run("return error for short PAN", func(t *testing.T) {
		pan := "123"
		_, err := MaskPAN(pan)
		if err == nil {
			t.Fatalf("expected error for short PAN, got nil")
		}
	})

	t.Run("return error for Invalid PAN", func(t *testing.T) {
		pan := "abcd"
		_, err := MaskPAN(pan)
		if err == nil {
			t.Fatalf("expected error for invalid PAN, got nil")
		}
	})
}