// Package search searches for a keyword within the html of pages (safe for concurrent use)
package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	// ErrURLEmpty to warn users that they passed an empty string in
	ErrURLEmpty = fmt.Errorf("the url string is empty")
	// ErrDomainMissing domain was missing from the url
	ErrDomainMissing = fmt.Errorf("url domain e.g .com, .net was missing")
)

// bufferPool maintains byte buffers used to read html content
type bufferPool struct {
	pool sync.Pool
}

// newbufferPool creates a new bufferPool bounded to the given size.
func newbufferPool(size int) *bufferPool {
	var bp bufferPool
	bp.pool.New = func() interface{} {
		return new(bytes.Buffer)
	}
	return &bp
}

// Get gets a Buffer from the bufferPool, or creates a new one if none are
// available in the pool.
func (bp *bufferPool) Get() *bytes.Buffer {
	return bp.pool.Get().(*bytes.Buffer)
}

// Put returns the given Buffer to the bufferPool.
func (bp *bufferPool) Put(b *bytes.Buffer) {
	b.Reset()
	bp.pool.Put(b)
}

// Scanner is the basic structure used to interact with the html content of the page
type Scanner struct {
	// used to limit the number of goroutines spinning up
	Sema chan struct{}
	// WasFound maps the url to whether or not the keyword was found
	WasFound map[string]bool

	buffer  *bufferPool // buffer used to read html content
	logging bool        // turn on and off logging
	mxt     sync.Mutex  // used internally to lock writing to the map
}

func normalizeURL(URL string) (s string, err error) {
	if URL == "" {
		err = ErrURLEmpty
		return
	}

	u, err := url.Parse(URL)
	if err != nil {
		return
	}

	scheme := u.Scheme
	path := u.Hostname()
	if path == "" {
		path = strings.Replace(u.Path, "/", "", -1)
	}

	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		err = ErrDomainMissing
		return
	}

	if scheme == "" {
		scheme = "http"
	}
	s = fmt.Sprintf("%s:%s", scheme, path)

	if !strings.Contains(path, "://") {
		s = fmt.Sprintf("%s://%s", scheme, path)
	}

	if strings.Count(u.Path, "/") > 1 {
		s += u.Path
	}

	return
}

// NewScanner returns a new scanner that takes a limit as a paramter to limit the number of goroutines spinning up
func NewScanner(limit int, enableLogging bool) *Scanner {
	return &Scanner{
		Sema:     make(chan struct{}, limit),
		WasFound: make(map[string]bool),
		logging:  enableLogging,
		buffer:   newbufferPool(limit / 2),
	}
}

func (sc *Scanner) writeToMap(URL string, found bool) {
	sc.mxt.Lock()
	sc.WasFound[URL] = found
	sc.mxt.Unlock()
}

// Search looks for the passed keyword in the html respose
func (sc *Scanner) Search(URL, keyword string) (err error) {
	// make sure to use the semaphore we've defined
	sc.Sema <- struct{}{}
	defer func() { <-sc.Sema }()

	if sc.logging {
		log.Infof("looking for the keyword %s in the url %s\n", keyword, URL)
	}

	URL, err = normalizeURL(URL)
	if err != nil {
		if sc.logging {
			log.Error(err)
		}
		return err
	}

	// not assuming a regex pattern will be passed
	var searchRegex *regexp.Regexp
	if strings.Contains(keyword, "(?i)") {
		searchRegex = regexp.MustCompile(keyword)
	} else {
		searchRegex = regexp.MustCompile("(?i)" + keyword)
	}

	var client = &http.Client{
		Timeout: time.Second * 10,
	}

	res, err := client.Get(URL)
	if err != nil {
		if sc.logging {
			log.Errorf("%v trying with https", err)
		}
		if !strings.Contains(URL, "https:") {
			URL = strings.Replace(URL, "http", "https", -1)
			res, err = client.Get(URL)
			if err != nil {
				if sc.logging {
					log.Errorf("%v https failed also", err)
				}
				sc.writeToMap(URL, false)
			}
			return err
		}
	}
	defer res.Body.Close()

	buf := sc.buffer.Get()
	io.Copy(buf, res.Body)
	defer sc.buffer.Put(buf)

	found := searchRegex.Match(buf.Bytes())
	sc.writeToMap(URL, found)
	return
}

// SearchWithRegx allows you to pass a regular expression i as a search paramter
func (sc *Scanner) SearchWithRegx(URL string, keyword *regexp.Regexp) (err error) {
	// make sure to use the semaphore we've defined
	sc.Sema <- struct{}{}
	defer func() { <-sc.Sema }()

	if sc.logging {
		log.Infof("looking for the keyword %s in the url %s\n", keyword, URL)
	}

	URL, err = normalizeURL(URL)
	if err != nil {
		if sc.logging {
			log.Error(err)
		}
		return err
	}

	var client = &http.Client{
		Timeout: time.Second * 10,
	}

	res, err := client.Get(URL)
	if err != nil {
		if sc.logging {
			log.Errorf("%v trying with https", err)
		}
		if !strings.Contains(URL, "https:") {
			URL = strings.Replace(URL, "http", "https", -1)
			res, err = client.Get(URL)
			if err != nil {
				if sc.logging {
					log.Errorf("%v https failed also", err)
				}
				sc.writeToMap(URL, false)
			}
			return err
		}
	}
	defer res.Body.Close()

	buf := sc.buffer.Get()
	io.Copy(buf, res.Body)
	defer sc.buffer.Put(buf)

	found := keyword.Match(buf.Bytes())
	sc.writeToMap(URL, found)
	return
}

// MapToIOReaderWriter converts the map of urls: bool to an io.Reader so that the end user can decide how they want that data
// csv, text, etc
func (sc *Scanner) MapToIOReaderWriter() (io.Reader, error) {
	sc.mxt.Lock()
	b, err := json.Marshal(sc.WasFound)
	sc.mxt.Unlock()
	if err != nil {
		if sc.logging {
			log.Error(err)
		}
		return nil, err
	}
	return bytes.NewReader(b), nil
}
