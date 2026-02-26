package config

import "testing"

func TestIsWeakAPIKey(t *testing.T) {
	cases := []struct {
		key  string
		weak bool
	}{
		{key: "", weak: true},
		{key: "short", weak: true},
		{key: "change-me", weak: true},
		{key: "replace-with-long-random-api-key", weak: true},
		{key: "dev-only-change-this-key-please", weak: false},
		{key: "n2H8x9fQ4mL7pRt2", weak: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			if got := isWeakAPIKey(tc.key); got != tc.weak {
				t.Fatalf("isWeakAPIKey(%q) = %v, want %v", tc.key, got, tc.weak)
			}
		})
	}
}

func TestTruthyFalsy(t *testing.T) {
	if !isTruthy("1") || !isTruthy("true") || !isTruthy("Yes") || !isTruthy("on") {
		t.Fatal("isTruthy() failed for truthy values")
	}
	if isTruthy("0") || isTruthy("off") || isTruthy("nope") {
		t.Fatal("isTruthy() returned true for falsy values")
	}

	if !isFalsy("0") || !isFalsy("false") || !isFalsy("NO") || !isFalsy("off") {
		t.Fatal("isFalsy() failed for falsy values")
	}
	if isFalsy("1") || isFalsy("on") || isFalsy("yes") {
		t.Fatal("isFalsy() returned true for truthy values")
	}
}
