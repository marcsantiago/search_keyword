package search

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"regexp"
	"sync"
	"testing"
)

func TestSortInterface(t *testing.T) {
	r := Results{Result{}, Result{}, Result{}}
	if r.Len() != len(r) {
		t.Errorf("lengths should be the same. Got %d wanted: %d", r.Len(), len(r))
	}

	if r.Less(0, 1) {
		t.Errorf("Less should be false because the elements are the same")
	}

	r.Swap(0, 1)
	if r[0] != r[1] {
		t.Errorf("elements are should be the same same")
	}

	r = Results{Result{URL: "blah"}, Result{URL: "bah"}, Result{}}
	if !r.Less(1, 0) {
		t.Errorf("Less should be true bah is les than blah")
	}

	r.Swap(0, 1)
	if r[0] == r[1] {
		t.Errorf("elements are should be the aren't the same")
	}
}

func TestNormalizeURL(t *testing.T) {
	var cases = []struct {
		Name string
		In   string
		Out  string
	}{
		{"no protocol", "facebook.com/", "http://facebook.com"},
		{"http protocol", "http://facebook.com/", "http://facebook.com"},
		{"https protocol", "https://facebook.com/", "https://facebook.com"},
		{"empty", "", ErrURLEmpty.Error()},
		{"no domain", "http://facebook", ErrDomainMissing.Error()},
		{"no domain or protocol", "facebook", ErrDomainMissing.Error()},
		{"long path", "https://en.wikipedia.org/wiki/Email_address", "https://en.wikipedia.org/wiki/Email_address"},
		{"bad url formating", "%2i23jr93udn.com", "parse %2i23jr93udn.com: invalid URL escape \"%2i\""},
	}

	for i, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			out, err := normalizeURL(c.In)
			if err == nil {
				if c.Out != out {
					t.Fatalf("test %d failed. expected %s got %s", i, c.Out, out)
				}
			} else {
				if c.Out != err.Error() {
					t.Fatalf("test %d failed. expected %s got %v", i, c.Out, err)
				}
			}
		})
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
	sc := NewScanner(1, 0, true)
	sc.testing = true
	err := sc.Search("facebook.com/", "Connect with friends")
	if err != nil {
		t.Error(err)
	}

	if len(sc.results) != 1 {
		t.Errorf("the map should only have one value in it, found %d", len(sc.results))
	}

	err = sc.Search("facebook.com/", "(?i)Connect with friends")
	if err != nil {
		t.Error(err)
	}
}

func TestScannerRegx(t *testing.T) {
	reg := regexp.MustCompile(`([a-z0-9!#$%&'*+\/=?^_{|}~-]+(?:\.[a-z0-9!#$%&'*+\/=?^_{|}~-]+)*(@|\sat\s)(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(\.|\sdot\s))+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)`)
	sc := NewScanner(1, 0, false)
	sc.testing = true
	err := sc.SearchWithRegx("https://en.wikipedia.org/wiki/Email_address", reg)
	if err != nil {
		t.Error(err)
	}
}

func TestSearchForEmailx(t *testing.T) {
	sc := NewScanner(1, 0, false)
	sc.testing = true
	err := sc.SearchForEmail("https://en.wikipedia.org/wiki/Email_address", nil, nil)
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%+v\n", sc.GetResults())
}

func TestResultsToReader(t *testing.T) {
	sc := NewScanner(1, 0, false)
	sc.testing = true
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

func TestGetResults(t *testing.T) {
	var results Results
	sc := NewScanner(1, 0, false)
	sc.testing = true
	if len(results) != len(sc.GetResults()) {
		t.Errorf("length of results should be equal to length sc.GetResults()")
	}
}

func TestLogging(t *testing.T) {
	sc := NewScanner(1, 0, true)
	sc.testing = true
	sc.Search("facebook.com", "sign up")
	sc.SearchWithRegx("facebook.com", regexp.MustCompile("(?i)sign up"))
}
