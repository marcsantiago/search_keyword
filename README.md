# search_keyword
interview question keeping company name secret to avoid copy and pasting ;-)

# search
`import "github.com/marcsantiago/search_keyword/search"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>
Package search searches for a keyword within the html of pages (safe for concurrent use)




## <a name="pkg-index">Index</a>
* [Variables](#pkg-variables)
* [type Scanner](#Scanner)
  * [func NewScanner(limit int, enableLogging bool) *Scanner](#NewScanner)
  * [func (sc *Scanner) MapToIOReaderWriter() (io.Reader, error)](#Scanner.MapToIOReaderWriter)
  * [func (sc *Scanner) Search(URL, keyword string) (err error)](#Scanner.Search)
  * [func (sc *Scanner) SearchWithRegx(URL string, keyword *regexp.Regexp) (err error)](#Scanner.SearchWithRegx)


#### <a name="pkg-files">Package files</a>
[search.go](/src/github.com/marcsantiago/search_keyword/search/search.go) 



## <a name="pkg-variables">Variables</a>
``` go
var (
    // ErrURLEmpty to warn users that they passed an empty string in
    ErrURLEmpty = fmt.Errorf("the url string is empty")
    // ErrDomainMissing domain was missing from the url
    ErrDomainMissing = fmt.Errorf("url domain e.g .com, .net was missing")
)
```



## <a name="Scanner">type</a> [Scanner](/src/target/search.go?s=1223:1586#L43)
``` go
type Scanner struct {
    // used to limit the number of goroutines spinning up
    Sema chan struct{}
    // WasFound maps the url to whether or not the keyword was found
    WasFound map[string]bool
    // contains filtered or unexported fields
}
```
Scanner is the basic structure used to interact with the html content of the page







### <a name="NewScanner">func</a> [NewScanner](/src/target/search.go?s=2296:2351#L94)
``` go
func NewScanner(limit int, enableLogging bool) *Scanner
```
NewScanner returns a new scanner that takes a limit as a paramter to limit the number of goroutines spinning up





### <a name="Scanner.MapToIOReaderWriter">func</a> (\*Scanner) [MapToIOReaderWriter](/src/target/search.go?s=5127:5186#L219)
``` go
func (sc *Scanner) MapToIOReaderWriter() (io.Reader, error)
```
MapToIOReaderWriter converts the map of urls: bool to an io.Reader so that the end user can decide how they want that data
csv, text, etc




### <a name="Scanner.Search">func</a> (\*Scanner) [Search](/src/target/search.go?s=2694:2752#L110)
``` go
func (sc *Scanner) Search(URL, keyword string) (err error)
```
Search looks for the passed keyword in the html respose




### <a name="Scanner.SearchWithRegx">func</a> (\*Scanner) [SearchWithRegx](/src/target/search.go?s=3985:4066#L168)
``` go
func (sc *Scanner) SearchWithRegx(URL string, keyword *regexp.Regexp) (err error)
```
SearchWithRegx allows you to pass a regular expression i as a search paramter








- - -
Generated by [godoc2md](http://godoc.org/github.com/davecheney/godoc2md)
