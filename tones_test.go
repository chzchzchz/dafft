package main

import "testing"

func TestHz2Tone(t *testing.T) {
	cases := []struct {
		hz   int
		want string
	}{
		{20, "???"},
		{33, "C1"},    // just above C1: nearest note is still C1
		{34, "C#1/Db1"}, // past the midpoint toward C#1
		{330, "E4"},   // regression: snapped to F4 when midpoint compare was broken
		{349.0, "F4"},
		{440, "A4"},
		{261, "C4"},
		{8000, "???"},
	}
	for _, c := range cases {
		if got := hz2tone(c.hz); got != c.want {
			t.Errorf("hz2tone(%d) = %q, want %q", c.hz, got, c.want)
		}
	}
}
