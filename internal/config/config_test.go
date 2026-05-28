package config

import "testing"

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "root", in: "https://wiki.example.com", want: "https://wiki.example.com/api"},
		{name: "root trailing slash", in: "https://wiki.example.com/", want: "https://wiki.example.com/api"},
		{name: "api", in: "https://wiki.example.com/api", want: "https://wiki.example.com/api"},
		{name: "api trailing slash", in: "https://wiki.example.com/api/", want: "https://wiki.example.com/api"},
		{name: "nested self hosted", in: "https://wiki.example.com/outline", want: "https://wiki.example.com/outline/api"},
		{name: "drops query and fragment", in: "https://wiki.example.com/?x=1#top", want: "https://wiki.example.com/api"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeBaseURL(test.in)
			if err != nil {
				t.Fatalf("NormalizeBaseURL() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("NormalizeBaseURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeBaseURLRejectsInvalidURL(t *testing.T) {
	for _, value := range []string{"", "wiki.example.com", "://wiki.example.com"} {
		if _, err := NormalizeBaseURL(value); err == nil {
			t.Fatalf("NormalizeBaseURL(%q) expected error", value)
		}
	}
}
