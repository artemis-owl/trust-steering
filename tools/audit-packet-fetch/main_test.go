package main

import "testing"

func TestEvidenceExpected(t *testing.T) {
	cases := []struct {
		name     string
		decision string
		want     bool
	}{
		{
			name:     "denied and executed by consumer",
			decision: `{"request":{"action":"inference","attributes":{"executor":"consumer"}},"policy":{"decision":"deny"}}`,
			want:     false,
		},
		{
			name:     "allowed and executed by consumer",
			decision: `{"request":{"action":"inference","attributes":{"executor":"consumer"}},"policy":{"decision":"allow"}}`,
			want:     true,
		},
		{
			name:     "denied without executor",
			decision: `{"request":{"action":"deploy"},"policy":{"decision":"deny"}}`,
			want:     true,
		},
		{
			name:     "not JSON",
			decision: `not json`,
			want:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evidenceExpected([]byte(tc.decision)); got != tc.want {
				t.Fatalf("evidenceExpected(%s) = %v, want %v", tc.decision, got, tc.want)
			}
		})
	}
}
