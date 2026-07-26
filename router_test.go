package main

import "testing"

func TestAcceptsGzip(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   bool
	}{
		{name: "empty", header: "", want: false},
		{name: "identity only", header: "identity", want: false},
		{name: "plain gzip", header: "gzip", want: true},
		{name: "gzip with weight", header: "gzip;q=1.0", want: true},
		{name: "gzip q zero", header: "gzip;q=0", want: false},
		{name: "gzip q zero with spaces", header: "gzip; q=0", want: false},
		{name: "case insensitive coding and q", header: "GZip;Q=0", want: false},
		{name: "case insensitive accept", header: "GZIP", want: true},
		{name: "star allows gzip", header: "*;q=1", want: true},
		{name: "star q zero", header: "*;q=0", want: false},
		{name: "explicit gzip beats star", header: "gzip;q=0, *;q=1", want: false},
		{name: "gzip after other codings", header: "deflate, gzip;q=0.5, br", want: true},
		{name: "malformed q rejected", header: "gzip;q=abc", want: false},
		{name: "q out of range rejected", header: "gzip;q=1.5", want: false},
		{name: "whitespace around tokens", header: "  gzip ; q = 0.8  ", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := acceptsGzip(tc.header); got != tc.want {
				t.Fatalf("acceptsGzip(%q) = %v, want %v", tc.header, got, tc.want)
			}
		})
	}
}
