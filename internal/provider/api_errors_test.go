package provider

import "testing"

func TestFormatAPIError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		want       string
	}{
		{
			name:       "structured API error",
			statusCode: 400,
			body:       []byte(`{"message":"invalid request","request_id":"req-123","timestamp":"2026-10-05T12:00:00Z","detail":"some error detail"}`),
			want:       "status: 400 Bad Request, detail: some error detail",
		},
		{
			name:       "empty or invalid body",
			statusCode: 500,
			body:       []byte("not-json"),
			want:       "status: 500 Internal Server Error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formatAPIError(test.statusCode, test.body); got != test.want {
				t.Errorf("formatAPIError() = %q, want %q", got, test.want)
			}
		})
	}
}
