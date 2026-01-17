package id

import "testing"

func TestIsInstanceID(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "valid lower", value: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", want: true},
		{name: "valid mixed", value: "AaBbCcDdEeFf00112233445566778899", want: true},
		{name: "invalid length", value: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", want: false},
		{name: "invalid char", value: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaag", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInstanceID(tt.value); got != tt.want {
				t.Fatalf("IsInstanceID(%q)=%v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
