package search

import (
	"bytes"
	"io/ioutil"
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
	}

	for i, c := range cases {
		out, _ := normalizeURL(c.In)

		if c.Out != out {
			t.Errorf("test %d failed. Excepted %s got %s", i, c.Out, out)
		}
	}
}

func TestNewBufferPoolResets(t *testing.T) {
	limit := 10
	pool := NewBufferPool(limit)
	var wg sync.WaitGroup
	readerCh := make(chan *bytes.Buffer, limit)
	for i := 0; i <= limit; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := pool.Get()
			buf.WriteByte(byte(i))
			readerCh <- buf
			defer pool.Put(buf)
		}()
	}

	go func() {
		wg.Wait()
		close(readerCh)
	}()

	for n := range readerCh {
		if n.Len() > 0 {
			t.Errorf("the buffer should be zero since it was put back on defer")
		}
	}
}

func TestNewBufferPool(t *testing.T) {
	limit := 10
	pool := NewBufferPool(limit)
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
	key := "http://facebook.com"
	sc := NewScanner(1, true)
	err := sc.Search("facebook.com/", "Connect with friends")
	if err != nil {
		t.Error(err)
	}

	if len(sc.WasFound) != 1 {
		t.Errorf("the map should only have one value in it, found %d", len(sc.WasFound))
	}

	if val, ok := sc.WasFound[key]; ok {
		if val != true {
			t.Errorf("connect with friends should have been found in the html")
		}
	} else {
		t.Errorf("key %s should have been present", key)
	}

	err = sc.Search("facebook.com/", "candyland")
	if err != nil {
		t.Error(err)
	}

	if val, ok := sc.WasFound[key]; ok {
		if val != false {
			t.Errorf("candyland should not have been marked as found")
		}
	} else {
		t.Errorf("key %s should have been present", key)
	}

}

func TestMapToIOReader(t *testing.T) {
	key := "http://facebook.com"
	sc := NewScanner(1, true)
	err := sc.Search("facebook.com/", "Connect with friends")
	if err != nil {
		t.Error(err)
	}

	if len(sc.WasFound) != 1 {
		t.Errorf("the map should only have one value in it, found %d", len(sc.WasFound))
	}

	if val, ok := sc.WasFound[key]; ok {
		if val != true {
			t.Errorf("connect with friends should have been found in the html")
		}
	} else {
		t.Errorf("key %s should have been present", key)
	}

	reader, err := sc.MapToIOReaderWriter()
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
