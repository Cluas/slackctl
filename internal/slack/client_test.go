package slack

import "testing"

func TestPercentEncodeCookie(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple ascii",
			input: "xoxd-abc123",
			want:  "xoxd-abc123",
		},
		{
			name:  "with slashes and plus",
			input: "xoxd-7K1/pOc+ZRH",
			want:  "xoxd-7K1%2FpOc%2BZRH",
		},
		{
			name:  "with equals",
			input: "xoxd-abc==",
			want:  "xoxd-abc%3D%3D",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "unreserved chars preserved",
			input: "abc-_.~!*'()",
			want:  "abc-_.~!*'()",
		},
		{
			name:  "space encoded as %20",
			input: "a b",
			want:  "a%20b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentEncodeCookie(tt.input)
			if got != tt.want {
				t.Errorf("percentEncodeCookie(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
