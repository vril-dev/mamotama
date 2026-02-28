package handler

import "testing"

func TestParseCountryBlockRaw_Valid(t *testing.T) {
	got, err := ParseCountryBlockRaw(`
# comments
jp
US
UNKNOWN
JP
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"JP", "UNKNOWN", "US"}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d want=%d got=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%s want=%s (all=%v)", i, got[i], want[i], got)
		}
	}
}

func TestParseCountryBlockRaw_Invalid(t *testing.T) {
	if _, err := ParseCountryBlockRaw("JPN\n"); err == nil {
		t.Fatal("expected error for non alpha-2 code")
	}
	if _, err := ParseCountryBlockRaw("U1\n"); err == nil {
		t.Fatal("expected error for non alphabetic code")
	}
	if _, err := ParseCountryBlockRaw("JP US\n"); err == nil {
		t.Fatal("expected error for multiple tokens per line")
	}
}
