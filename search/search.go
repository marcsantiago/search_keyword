// Package search searches for a keyword within the html of pages (safe for concurrent use)
package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fatih/color"
	log "github.com/marcsantiago/logger"
)

var (
	// DefaultTimeout is the duration used to determine get request timeout
	// this is exported so that i can be changed
	DefaultTimeout = 10 * time.Second

	// ErrURLEmpty to warn users that they passed an empty string in
	ErrURLEmpty = fmt.Errorf("the url string is empty")
	// ErrDomainMissing domain was missing from the url
	ErrDomainMissing = fmt.Errorf("url domain e.g .com, .net was missing")
	// ErrUnresolvedOrTimedOut ...
	ErrUnresolvedOrTimedOut = fmt.Errorf("url could not be resolved or timeout")

	// EmailRegex provides a base email regex for scraping emails
	EmailRegex = regexp.MustCompile(`([a-z0-9!#$%&'*+\/=?^_{|}~-]+(?:\.[a-z0-9!#$%&'*+\/=?^_{|}~-]+)*(@|\sat\s)(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(\.|\sdot\s))+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)`)

	searchTermColor = color.New(color.FgGreen).SprintFunc()
	foundColor      = color.New(color.FgGreen).SprintFunc()
	notFoundColor   = color.New(color.FgRed).SprintFunc()
	newLineReplacer = strings.NewReplacer("\r\n", "", "\n", "", "\r", "")
	logkey          = "Scanner"
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

// Result is the basic return type for Search and SearchWithRegx
type Result struct {
	// Keyword is the passed keyword. It is an interface because it can be a string or regular expression
	Keyword interface{}
	// URL is the url passed in
	URL string
	// Found determines whether or not the keyword was matched on the page
	Found   bool
	Context interface{}
}

// Results is the plural of results which implements the Sort interface. Sorting by URL.  If the slice needs to be sorted then the user can call sort.Sort
type Results []Result

func (slice Results) Len() int {
	return len(slice)
}

func (slice Results) Less(i, j int) bool {
	return slice[i].URL < slice[j].URL
}

func (slice Results) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

// Scanner is the basic structure used to interact with the html content of the page
type Scanner struct {
	// client is used to make requests
	client *http.Client
	// sema is used to limit the number of goroutines spinning up
	sema sema
	// results is a slice of result
	results Results
	// buffer used to read html content
	buffer *bufferPool
	// turn on and off logging
	logging bool
	// used internally to lock writing to the map
	mxt sync.Mutex
	// used to define depth of search
	depthLimit int
	// used to turn off logging in testing
	testing bool
}

type sema chan struct{}

func (s sema) release() { <-s }
func (s sema) load()    { s <- struct{}{} }

func inSlice(tar string, s []string) bool {
	for _, i := range s {
		if tar == i {
			return true
		}
	}
	return false
}

func linksToCheck(baseURL string, limit int) (moreURLS []string) {
	moreURLS = []string{baseURL}
	if limit == 0 {
		return
	}

	doc, err := goquery.NewDocument(baseURL)
	if err != nil {
		log.Error(logkey, "Could not create doc", "error", err)
		return
	}
	doc.Find("body a").Each(func(index int, item *goquery.Selection) {
		link, _ := item.Attr("href")
		if strings.Contains(link, baseURL) {
			if !inSlice(link, moreURLS) {
				moreURLS = append(moreURLS, link)
			}
		}
		if len(moreURLS) >= limit {
			return
		}
	})
	return
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
func NewScanner(concurrentLimit, depthLimit int, enableLogging bool) *Scanner {
	return &Scanner{
		client: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				Dial: (&net.Dialer{
					Timeout: DefaultTimeout,
				}).Dial,
				TLSHandshakeTimeout: DefaultTimeout,
				MaxIdleConns:        concurrentLimit,
				MaxIdleConnsPerHost: concurrentLimit,
			},
			Timeout: DefaultTimeout,
		},
		depthLimit: depthLimit,
		sema:       make(sema, concurrentLimit),
		logging:    enableLogging,
		buffer:     newbufferPool(concurrentLimit / 2),
	}
}

func (sc *Scanner) saveResult(URL string, keyword interface{}, found bool, chunk interface{}) {
	sc.mxt.Lock()
	if sc.testing {
		if found {
			log.Info(logkey, "Result", "Search term", searchTermColor(keyword), "found", foundColor("yes"), "url", URL)
		} else {
			log.Info(logkey, "Result", "Search term", searchTermColor(keyword), "found", notFoundColor("no"), "url", URL)
		}
	}

	sc.results = append(sc.results, Result{URL: URL, Found: found, Keyword: keyword, Context: chunk})
	sc.mxt.Unlock()
	return
}

// Search looks for the passed keyword in the html respose
func (sc *Scanner) Search(URL, keyword string) (err error) {
	sc.sema.load()
	defer sc.sema.release()

	URL, err = normalizeURL(URL)
	if err != nil {
		if sc.logging {
			log.Error(logkey, "Could not normalize url", "error", err)
		}
		return err
	}

	// not assuming a regex pattern will be passed
	var searchRegex, contextRegex *regexp.Regexp
	if strings.Contains(keyword, "(?i)") {
		searchRegex = regexp.MustCompile(keyword)
		contextRegex = regexp.MustCompile(fmt.Sprintf("(?i)(<[^<]+)(%s)([^>]+>)", strings.Replace(keyword, "(?i)", "", 1)))
	} else {
		searchRegex = regexp.MustCompile("(?i)" + keyword)
		contextRegex = regexp.MustCompile(fmt.Sprintf("(?i)(<[^<]+)(%s)([^>]+>)", keyword))
	}

	urls := linksToCheck(URL, sc.depthLimit)
	for _, url := range urls {
		if sc.logging {
			log.Info(logkey, "Looking for keyword", "keyword", keyword, "url", url)
		}

		var shouldTestSSL bool
		var shouldErrorOut bool

		res, err := sc.client.Get(url)
		if err != nil {
			if sc.logging {
				log.Error(logkey, "http failed", "error", err)
			}
			shouldTestSSL = true
		}

		if shouldTestSSL {
			if !strings.Contains(url, "https:") {
				url = strings.Replace(url, "http", "https", 1)
				res, err = sc.client.Get(url)
				if err != nil {
					if sc.logging {
						log.Error(logkey, "https failed", "error", err)
					}
					shouldErrorOut = true
				}
			}
		}

		if shouldErrorOut {
			sc.saveResult(url, keyword, false, "")
			return ErrUnresolvedOrTimedOut
		}
		defer res.Body.Close()

		buf := sc.buffer.Get()
		defer sc.buffer.Put(buf)
		io.Copy(buf, res.Body)

		b := buf.Bytes()
		found := searchRegex.Match(b)
		var context string
		if found {
			context = newLineReplacer.Replace(string(contextRegex.Find(b)))
		}
		sc.saveResult(url, keyword, found, context)
	}

	return nil
}

// SearchForEmail returns possible emails from the source pages.  If you do not provide a regex it will use the default value
// defined in the var EmailRegex, if you wish to filter finds, add a filter slice otherwise everything is can find will be dumped
func (sc *Scanner) SearchForEmail(URL string, emailRegex *regexp.Regexp, filters []string) (err error) {
	if emailRegex == nil {
		emailRegex = EmailRegex
	}

	// make sure to use the semaphore we've defined
	sc.sema.load()
	defer sc.sema.release()

	URL, err = normalizeURL(URL)
	if err != nil {
		if sc.logging {
			log.Error(logkey, "Could not normalize URL", "error", err)
		}
		return err
	}

	urls := linksToCheck(URL, sc.depthLimit)
	for _, url := range urls {
		if sc.logging {
			log.Info(logkey, "Looking for the a email", "url", url)
		}
		res, err := sc.client.Get(url)
		if err != nil {
			if sc.logging {
				log.Info(logkey, "Trying with https", "error", err)
			}
			if !strings.Contains(url, "https:") {
				url = strings.Replace(url, "http", "https", -1)
				res, err = sc.client.Get(url)
				if err != nil {
					if sc.logging {
						log.Error(logkey, "https failed also", "error", err)
					}
					sc.saveResult(url, "", false, "")
				}
				return ErrUnresolvedOrTimedOut
			}
		}
		defer res.Body.Close()

		buf := sc.buffer.Get()
		defer sc.buffer.Put(buf)
		io.Copy(buf, res.Body)

		emails := emailRegex.FindStringSubmatch(buf.String())
		var clean []string
		found := false
		if len(emails) > 0 {
			found = true

			for _, e := range emails {
				if len(filters) > 0 {
					for _, f := range filters {
						if !strings.Contains(e, f) && !inSlice(e, clean) && len(e) > 1 {
							clean = append(clean, e)
						}
					}
				} else {
					if len(e) > 1 && !inSlice(e, clean) {
						clean = append(clean, e)
					}
				}

			}
		}
		sc.saveResult(url, "", found, clean)
	}
	return
}

// SearchWithRegx allows you to pass a regular expression i as a search paramter
func (sc *Scanner) SearchWithRegx(URL string, keyword *regexp.Regexp) (err error) {
	sc.sema.load()
	defer sc.sema.release()

	if sc.logging {
		log.Info(logkey, "Looking for the keyword", "keyword", keyword, "url", URL)
	}

	URL, err = normalizeURL(URL)
	if err != nil {
		if sc.logging {
			log.Error(logkey, "Could not normalize urk", "error", err)
		}
		return err
	}

	res, err := sc.client.Get(URL)
	if err != nil {
		if sc.logging {
			log.Info(logkey, "Trying with https", "error", err)
		}
		if !strings.Contains(URL, "https:") {
			URL = strings.Replace(URL, "http", "https", -1)
			res, err = sc.client.Get(URL)
			if err != nil {
				if sc.logging {
					log.Error(logkey, "https failed also", "error", err)
				}
				sc.saveResult(URL, keyword, false, "")
			}
			return err
		}
	}
	defer res.Body.Close()

	buf := sc.buffer.Get()
	defer sc.buffer.Put(buf)
	io.Copy(buf, res.Body)

	b := buf.Bytes()
	found := keyword.Match(b)
	var context string
	if found {
		contextRegex := regexp.MustCompile(fmt.Sprintf("(?i)(<[^<]+)(%s)([^>]+>)", keyword))
		context = newLineReplacer.Replace(string(contextRegex.Find(b)))
	}
	sc.saveResult(URL, keyword, found, context)
	return
}

// ResultsToReader sorts a slice of Result to an io.Reader so that the end user can decide how they want that data
// csv, text, etc
func (sc *Scanner) ResultsToReader() (io.Reader, error) {
	b, err := json.Marshal(sc.results)
	if err != nil {
		if sc.logging {
			log.Error(logkey, "Could not marshal data", "error", err)
		}
		return nil, err
	}
	return bytes.NewReader(b), nil
}

// GetResults returns raw results not converted to a io.Reader
func (sc *Scanner) GetResults() Results {
	return sc.results
}
