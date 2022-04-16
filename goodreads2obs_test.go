package main

import (
	"bytes"
	"testing"
)

func TestRemoveRandom(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"average: 1.2", ""},
		{"pre\naverage: 3.45\npost", "pre\n\npost"},
		{"pre\npages: 123\npost", "pre\n\npost"},
		{"average: 3.45\npages: 123\n", "\n\n"},
	}
	for _, test := range tests {
		got := removeRandom([]byte(test.in))

		if !bytes.Equal(got, []byte(test.want)) {
			t.Errorf("%q -> %q, want %q\n", string(test.in), string(got), string(test.want))
		}
	}
}
