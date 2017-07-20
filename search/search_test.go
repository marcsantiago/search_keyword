package search

import "testing"

func TestNormalizeURL(t *testing.T) {
	var cases = []struct {
		In  string
		Out string
	}{
		{"facebook.com/", "http://facebook.com"},
		{"http://facebook.com/", "http://facebook.com"},
		{"https://facebook.com/", "https://facebook.com"},
		{"", ""},
		{"facebook", ""},
	}

	for i, c := range cases {
		out, _ := normalizeURL(c.In)

		if c.Out != out {
			t.Errorf("test %d failed. Excepted %s got %s", i, c.Out, out)
		}
	}

}
