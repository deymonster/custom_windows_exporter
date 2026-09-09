//go:build linux

package linux

import "testing"

func TestClassifySensorReading(t *testing.T) {
	tests := []struct {
		name   string
		status string
		value  string
		want   string
	}{
		{"normal", "ok", "31 degrees C", "healthy"},
		{"unpopulated fan", "lnr", "0 RPM", "needs_baseline"},
		{"unpopulated thermal", "lnr", "0 degrees C", "needs_baseline"},
		{"nonzero lower critical", "lnr", "400 RPM", "fault"},
		{"unavailable", "ns", "no reading", "unavailable"},
		{"intrusion", "ok", "Drive Bay intrusion", "healthy"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifySensorReading(test.status, test.value); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
