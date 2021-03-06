# search_keyword
interview question keeping company name secret to avoid copy and pasting ;-)
*Example Use:*
`go run main.go -in example_input_and_output/urls.txt -out example_input_and_output/results.txt -keyword "sign up"`

# search
`import "github.com/marcsantiago/search_keyword/search"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>
Package search searches for a keyword within the html of pages (safe for concurrent use)




## <a name="pkg-index">Index</a>
* [Variables](#pkg-variables)
* [type Result](#Result)
* [type Results](#Results)
  * [func (slice Results) Len() int](#Results.Len)
  * [func (slice Results) Less(i, j int) bool](#Results.Less)
  * [func (slice Results) Swap(i, j int)](#Results.Swap)
* [type Scanner](#Scanner)
  * [func NewScanner(limit int, enableLogging bool) *Scanner](#NewScanner)
  * [func (sc *Scanner) ResultsToReader() (io.Reader, error)](#Scanner.ResultsToReader)
  * [func (sc *Scanner) Search(URL, keyword string) (err error)](#Scanner.Search)


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



## <a name="Result">type</a> [Result](/src/target/search.go?s=1203:1474#L43)
``` go
type Result struct {
    // Keyword is the passed keyword. It is an interface because it can be a string or regular expression
    Keyword interface{}
    // URL is the url passed in
    URL string
    // Found determines whether or not the keyword was matched on the page
    Found bool
}
```
Result is the basic return type for Search and SearchWithRegex










## <a name="Results">type</a> [Results](/src/target/search.go?s=1631:1652#L53)
``` go
type Results []Result
```
Results is the plural of results which implements the Sort interface. Sorting by URL.  If the slice needs to be sorted then the user can call sort.Sort










### <a name="Results.Len">func</a> (Results) [Len](/src/target/search.go?s=1654:1684#L55)
``` go
func (slice Results) Len() int
```



### <a name="Results.Less">func</a> (Results) [Less](/src/target/search.go?s=1709:1749#L59)
``` go
func (slice Results) Less(i, j int) bool
```



### <a name="Results.Swap">func</a> (Results) [Swap](/src/target/search.go?s=1791:1826#L63)
``` go
func (slice Results) Swap(i, j int)
```



## <a name="Scanner">type</a> [Scanner](/src/target/search.go?s=1958:2276#L68)
``` go
type Scanner struct {
    // contains filtered or unexported fields
}
```
Scanner is the basic structure used to interact with the html content of the page







### <a name="NewScanner">func</a> [NewScanner](/src/target/search.go?s=2986:3041#L121)
``` go
func NewScanner(limit int, enableLogging bool) *Scanner
```
NewScanner returns a new scanner that takes a limit as a paramter to limit the number of goroutines spinning up




### <a name="Scanner.ResultsToReader">func</a> (\*Scanner) [ResultsToReader](/src/target/search.go?s=5882:5937#L245)
``` go
func (sc *Scanner) ResultsToReader() (io.Reader, error)
```
ResultsToReader sorts a slice of Result to an io.Reader so that the end user can decide how they want that data
csv, text, etc




### <a name="Scanner.Search">func</a> (\*Scanner) [Search](/src/target/search.go?s=3424:3482#L136)
``` go
func (sc *Scanner) Search(URL, keyword string) (err error)
```
Search looks for the passed keyword in the html respose


- - -
Generated by [godoc2md](http://godoc.org/github.com/davecheney/godoc2md)
