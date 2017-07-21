package search

import (
	"bytes"
	"io/ioutil"
	"regexp"
	"sync"
	"testing"
)

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
		{"https://en.wikipedia.org/wiki/Email_address", "https://en.wikipedia.org/wiki/Email_address"},
	}

	for i, c := range cases {
		out, _ := normalizeURL(c.In)

		if c.Out != out {
			t.Errorf("test %d failed. Excepted %s got %s", i, c.Out, out)
		}
	}
}

func TestNormalizeURLDomainMissing(t *testing.T) {
	var cases = []struct {
		In  string
		Out string
	}{
		{"facebook", ""},
	}

	for _, c := range cases {
		_, err := normalizeURL(c.In)
		if err != ErrDomainMissing {
			t.Errorf("err got: %v wanted: %v", err, ErrDomainMissing)
		}
	}
}

func TestNormalizeURLEmpty(t *testing.T) {
	var cases = []struct {
		In  string
		Out string
	}{
		{"", ""},
	}

	for _, c := range cases {
		_, err := normalizeURL(c.In)
		if err != ErrURLEmpty {
			t.Errorf("err got: %v wanted: %v", err, ErrDomainMissing)
		}
	}
}

func TestNewBufferPool(t *testing.T) {
	limit := 10
	pool := newbufferPool(limit)
	var wg sync.WaitGroup
	readerCh := make(chan *bytes.Buffer, limit)
	for i := 0; i <= limit; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := pool.Get()
			buf.WriteByte(byte(i))
			readerCh <- buf
		}()
	}

	go func() {
		wg.Wait()
		close(readerCh)
	}()

	for n := range readerCh {
		if n.Len() < 0 {
			t.Errorf("the buffer hold data i did not put the buffer back")
		}
	}
}

func TestScanner(t *testing.T) {
	sc := NewScanner(1, false)
	err := sc.Search("facebook.com/", "Connect with friends")
	if err != nil {
		t.Error(err)
	}

	if len(sc.results) != 1 {
		t.Errorf("the map should only have one value in it, found %d", len(sc.results))
	}

	err = sc.Search("facebook.com/", "candyland")
	if err != nil {
		t.Error(err)
	}
}

func TestScannerRegx(t *testing.T) {
	reg := regexp.MustCompile(`([a-z0-9!#$%&'*+\/=?^_{|}~-]+(?:\.[a-z0-9!#$%&'*+\/=?^_{|}~-]+)*(@|\sat\s)(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(\.|\sdot\s))+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)`)
	sc := NewScanner(1, false)
	err := sc.SearchWithRegx("https://en.wikipedia.org/wiki/Email_address", reg)
	if err != nil {
		t.Error(err)
	}
}

func TestResultsToReader(t *testing.T) {
	sc := NewScanner(1, false)
	err := sc.Search("facebook.com/", "Connect with friends")
	if err != nil {
		t.Error(err)
	}

	reader, err := sc.ResultsToReader()
	if err != nil {
		t.Error(err)
	}

	b, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Error(err)
	}

	if len(b) == 0 {
		t.Errorf("bytes should not be 0")
	}
}
