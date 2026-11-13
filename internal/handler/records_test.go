package handler

import (
	"testing"

	"ns116/internal/model"
)

func TestValidateChange(t *testing.T) {
	const zone = "example.com."
	ok := func(name, typ string, ttl int64, values ...string) model.RecordChangeRequest {
		return model.RecordChangeRequest{Action: "CREATE", Name: name, Type: typ, TTL: ttl, Values: values}
	}
	cases := []struct {
		desc  string
		req   model.RecordChangeRequest
		valid bool
	}{
		{"A", ok("www.example.com.", "A", 300, "1.2.3.4"), true},
		{"AAAA", ok("www.example.com.", "AAAA", 300, "2001:db8::1"), true},
		{"apex A", ok("example.com.", "A", 300, "1.2.3.4"), true},
		{"wildcard TXT", ok("*.example.com.", "TXT", 0, `"v=spf1 -all"`), true},
		{"delegation NS", ok("sub.example.com.", "NS", 300, "ns1.other.net."), true},
		{"mixed case name", ok("WWW.Example.COM.", "CNAME", 300, "target.example.net."), true},
		{"type missing from the form", ok("example.com.", "DS", 300, "12345 13 2 abcdef"), true},
		{"SOA", ok("example.com.", "SOA", 900, "ns. admin. 1 2 3 4 5"), false},
		{"unknown type", ok("www.example.com.", "<script>", 300, "x"), false},
		{"apex NS", ok("example.com.", "NS", 300, "ns1.evil.net."), false},
		{"other zone", ok("www.other.com.", "A", 300, "1.2.3.4"), false},
		{"suffix lookalike", ok("notexample.com.", "A", 300, "1.2.3.4"), false},
		{"negative TTL", ok("www.example.com.", "A", -1, "1.2.3.4"), false},
		{"huge TTL", ok("www.example.com.", "A", 1<<31, "1.2.3.4"), false},
		{"no values", ok("www.example.com.", "A", 300), false},
		{"blank value", ok("www.example.com.", "TXT", 300, "  "), false},
		{"A with IPv6", ok("www.example.com.", "A", 300, "2001:db8::1"), false},
		{"AAAA with IPv4", ok("www.example.com.", "AAAA", 300, "1.2.3.4"), false},
		{"A with garbage", ok("www.example.com.", "A", 300, "1.2.3.4, 5.6.7.8"), false},
	}
	for _, c := range cases {
		if err := validateChange(c.req, zone); (err == nil) != c.valid {
			t.Errorf("%s: valid=%v, got err=%v", c.desc, c.valid, err)
		}
	}
}

func TestQualifyName(t *testing.T) {
	const zone = "example.com."
	for in, want := range map[string]string{
		"":                 "example.com.",
		"@":                "example.com.",
		"www":              "www.example.com.",
		"www.example.com":  "www.example.com.",
		"www.example.com.": "www.example.com.",
		"example.com":      "example.com.",
		"notexample.com":   "notexample.com.example.com.",
	} {
		if got := qualifyName(in, zone); got != want {
			t.Errorf("qualifyName(%q) = %q, want %q", in, got, want)
		}
	}
}
