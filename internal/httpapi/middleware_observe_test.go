package httpapi

import "testing"

func TestStatusClass(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{200, "2xx"},
		{204, "2xx"},
		{301, "3xx"},
		{400, "4xx"},
		{409, "4xx"},
		{500, "5xx"},
		{0, "other"},
	}
	for _, tt := range tests {
		if got := statusClass(tt.code); got != tt.want {
			t.Fatalf("statusClass(%d)=%q want %q", tt.code, got, tt.want)
		}
	}
}
