package reasons

import "testing"

func TestReason(t *testing.T) {
	tests := []struct {
		name string
		lang string
		want string
	}{
		{name: "default", lang: "", want: reasonByLang["go"]},
		{name: "go", lang: "go", want: reasonByLang["go"]},
		{name: "python", lang: "python", want: reasonByLang["python"]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Reason(tt.lang); got != tt.want {
				t.Fatalf("Reason(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}
