package main

import "testing"

func TestHostAuthenticationIsOptionalAndRequiresCompleteKey(t *testing.T) {
	for _, test := range []struct {
		value   string
		length  int
		wantErr bool
	}{
		{"", 0, false},
		{"00000000000000000000000000000000", 16, false},
		{"00", 0, true},
		{"0000000000000000000000000000000000", 0, true},
		{"invalid", 0, true},
	} {
		key, err := parseHostKey(test.value)
		if (err != nil) != test.wantErr || len(key) != test.length {
			t.Fatalf("input length=%d key length=%d error=%v", len(test.value), len(key), err)
		}
	}
}
