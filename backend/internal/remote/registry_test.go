package remote

import "testing"

func TestValidateCreateInstance(t *testing.T) {
	tests := []struct {
		name  string
		input CreateInstance
		want  bool
	}{
		{"valid", CreateInstance{Name: "Site A", URL: "https://example.com", APIKey: "key", PollIntervalS: 60}, true},
		{"missing api key", CreateInstance{Name: "Site A", URL: "https://example.com"}, false},
		{"invalid url", CreateInstance{Name: "Site A", URL: "example.com", APIKey: "key"}, false},
		{"invalid interval", CreateInstance{Name: "Site A", URL: "https://example.com", APIKey: "key", PollIntervalS: 5}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Validate(test.input) == nil; got != test.want {
				t.Fatalf("Validate() success = %v, want %v", got, test.want)
			}
		})
	}
}
