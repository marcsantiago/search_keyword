// Package search searches for a keyword within the html of pages (safe for concurrent use)
package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	log "github.com/marcsantiago/logger"
)

var (
	// DefaultTimeout is the duration used to determine get request timeout
	// this is exported so that I can be changed
	DefaultTimeout = 10 * time.Second
	// ErrURLEmpty to warn users that they passed an empty string in
	ErrURLEmpty = fmt.Errorf("url string is empty")
	// ErrDomainMissing domain was missing from the url
	ErrDomainMissing = fmt.Errorf("url domain e.g .com, .net was missing")
	// ErrUnresolvedOrTimedOut ...
	ErrUnresolvedOrTimedOut = fmt.Errorf("url could not be resolved or timed out")
	// EmailRegex provides a base email regex for scraping emails
	EmailRegex = regexp.MustCompile(`([a-z0-9!#$%&'*+\/=?^_{|}~-]+(?:\.[a-z0-9!#$%&'*+\/=?^_{|}~-]+)*(@|\sat\s)(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(\.|\sdot\s))+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)`)

	logkey          = "Scanner"
	newLineReplacer = strings.NewReplacer("\r\n", "", "\n", "", "\r", "")
)

// Result is the basic return type for Search and SearchWithRegx
type Result struct {
	// Keyword is the passed keyword. It is an interface because it can be a string or regular expression
	Keyword interface{} `json:"keyword,omitempty"`
	// URL is the url passed in
	URL string `json:"url,omitempty"`
	// Found determines whether or not the keyword was matched on the page
	Found   bool        `json:"found,omitempty"`
	Context interface{} `json:"context,omitempty"`
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
	// Client is used to make requests
	Client *http.Client
	// Semaphore is used to limit the number of goroutines spinning up
	Semaphore Semaphore
	// Sema is a slice of result
	Results Results
	// Logging turn on or off
	Logging bool
	// used internally to lock writing to the map
	mxt sync.Mutex
	// DepthLimit used to define depth of search
	DepthLimit int
}

// Semaphore ...
type Semaphore chan struct{}

func (s Semaphore) release() { <-s }
func (s Semaphore) load()    { s <- struct{}{} }

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
		log.Error(logkey, "could not create doc", "error", err)
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
		Client: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				Dial: (&net.Dialer{
					Timeout: DefaultTimeout,
				}).Dial,
				TLSHandshakeTimeout: DefaultTimeout,
				MaxIdleConns:        concurrentLimit * 2,
				MaxIdleConnsPerHost: concurrentLimit * 2,
			},
			Timeout: DefaultTimeout,
		},
		DepthLimit: depthLimit,
		Semaphore:  make(Semaphore, concurrentLimit),
		Logging:    enableLogging,
	}
}

func (sc *Scanner) saveResult(URL string, keyword interface{}, found bool, chunk interface{}) {
	if sc.Logging {
		foundS := "no"
		if found {
			foundS = "yes"
		}
		log.Info(logkey, "result", "search term", keyword, "found", foundS, "url", URL)
	}

	sc.mxt.Lock()
	sc.Results = append(sc.Results, Result{URL: URL, Found: found, Keyword: keyword, Context: chunk})
	sc.mxt.Unlock()
	return
}

// Search looks for the passed keyword in the html respose
func (sc *Scanner) Search(URL, keyword string) (err error) {
	sc.Semaphore.load()
	defer sc.Semaphore.release()

	URL, err = normalizeURL(URL)
	if err != nil {
		if sc.Logging {
			log.Error(logkey, "could not normalize url", "error", err)
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

	urls := linksToCheck(URL, sc.DepthLimit)
	for _, URL := range urls {
		if sc.Logging {
			log.Info(logkey, "looking for keyword", "keyword", keyword, "url", URL)
		}

		body, err := sc.makeRequest(URL)
		if err != nil {
			if strings.Contains(URL, "https:") {
				return err
			}
			URL = strings.Replace(URL, "http", "https", 1)
			body, err = sc.makeRequest(URL)
			if err != nil {
				return err
			}
		}

		found := searchRegex.Match(body)
		var context string
		if found {
			context = newLineReplacer.Replace(string(contextRegex.Find(body)))
		}
		sc.saveResult(URL, keyword, found, context)
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
	sc.Semaphore.load()
	defer sc.Semaphore.release()

	URL, err = normalizeURL(URL)
	if err != nil {
		if sc.Logging {
			log.Error(logkey, "could not normalize URL", "error", err)
		}
		return err
	}

	urls := linksToCheck(URL, sc.DepthLimit)
	for _, URL := range urls {
		if sc.Logging {
			log.Info(logkey, "looking for the a email", "url", URL)
		}

		body, err := sc.makeRequest(URL)
		if err != nil {
			if strings.Contains(URL, "https:") {
				return err
			}
			URL = strings.Replace(URL, "http", "https", 1)
			body, err = sc.makeRequest(URL)
			if err != nil {
				return err
			}
		}

		emails := emailRegex.FindStringSubmatch(string(body))
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
		sc.saveResult(URL, "", found, clean)
	}
	return
}

// SearchWithRegx allows you to pass a regular expression i as a search paramter
func (sc *Scanner) SearchWithRegx(URL string, keyword *regexp.Regexp) (err error) {
	sc.Semaphore.load()
	defer sc.Semaphore.release()

	if sc.Logging {
		log.Info(logkey, "looking for the keyword", "keyword", keyword, "url", URL)
	}

	URL, err = normalizeURL(URL)
	if err != nil {
		if sc.Logging {
			log.Error(logkey, "could not normalize urk", "error", err)
		}
		return err
	}

	body, err := sc.makeRequest(URL)
	if err != nil {
		if strings.Contains(URL, "https:") {
			return err
		}
		URL = strings.Replace(URL, "http", "https", 1)
		body, err = sc.makeRequest(URL)
		if err != nil {
			return err
		}
	}

	found := keyword.Match(body)
	var context string
	if found {
		contextRegex := regexp.MustCompile(fmt.Sprintf("(?i)(<[^<]+)(%s)([^>]+>)", keyword))
		context = newLineReplacer.Replace(string(contextRegex.Find(body)))
	}
	sc.saveResult(URL, keyword, found, context)
	return
}

// ResultsToReader sorts a slice of Result to an io.Reader so that the end user can decide how they want that data
// csv, text, etc
func (sc *Scanner) ResultsToReader() (io.Reader, error) {
	b, err := json.Marshal(sc.Results)
	if err != nil {
		if sc.Logging {
			log.Error(logkey, "could not marshal data", "error", err)
		}
		return nil, err
	}
	return bytes.NewReader(b), nil
}

func (sc *Scanner) makeRequest(URL string) ([]byte, error) {
	res, err := sc.Client.Get(URL)
	if err != nil {
		return []byte(""), err
	}
	defer res.Body.Close()
	return ioutil.ReadAll(res.Body)
}
