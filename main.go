// This is an example of how to use the parser. There are many more functions within the package that aren't being used here.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/fatih/color"
	log "github.com/marcsantiago/logger"
	"github.com/marcsantiago/search_keyword/search"
)

const logKey = "Main"

var errColor = color.New(color.FgRed).SprintFunc()

func readFromDirectory(dir, keyword string, sc *search.Scanner) (err error) {
	var wg sync.WaitGroup
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return
	}

	for _, f := range files {
		name := f.Name()
		p := path.Join(dir, name)

		// avoid .DS_Store and like files
		if strings.HasPrefix(name, ".") {
			continue
		}

		file, err := os.Open(p)
		if err != nil {
			log.Fatal(logKey, "couldn't open file", "error", err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			wg.Add(1)
			go scan(scanner.Text(), keyword, &wg, sc)
		}

		if err := scanner.Err(); err != nil {
			return err
		}
	}
	wg.Wait()
	return
}

func readFromFile(path, keyword string, sc *search.Scanner) (err error) {
	var wg sync.WaitGroup
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		wg.Add(1)
		go scan(scanner.Text(), keyword, &wg, sc)
	}
	if err = scanner.Err(); err != nil {
		return
	}
	wg.Wait()
	return
}

func scan(line, keyword string, wg *sync.WaitGroup, sc *search.Scanner) {
	defer wg.Done()

	parts := strings.Split(line, ",")
	if len(parts) <= 0 {
		return
	}

	URL := strings.Replace(parts[1], "\"", "", -1)
	err := sc.Search(URL, keyword)
	if err != nil {
		log.Error(logKey, "search error", "error", errColor(err))
	}
}

// this particular main function is written in such a way to satisfy
// the questions requirement, however the package search was written to
// be more generic
func main() {
	inputFile := flag.String("in", "", "the input file path containing the list of urls or folder path containing files pointing to urls")
	outFile := flag.String("out", "", "output file path")
	keyword := flag.String("keyword", "", "keyword to search for")
	enableLogging := flag.Bool("logging", false, "enables logging")
	limit := flag.Int("concurrency", 20, "set the limit of goroutines to spin up")
	depth := flag.Int("depth", 0, "set how depth of the search")
	flag.Parse()

	if *inputFile == "" {
		flag.PrintDefaults()
		log.Fatal(logKey, "input file path cannot be empty")
	}

	if *outFile == "" {
		flag.PrintDefaults()
		log.Fatal(logKey, "out file path cannot be empty")
	}

	if *keyword == "" {
		flag.PrintDefaults()
		log.Fatal(logKey, "keyword cannot be empty")
	}

	fi, err := os.Stat(*inputFile)
	if err != nil {
		log.Fatal(logKey, "os.Stat", "error", err)
	}

	sc := search.NewScanner(*limit, *depth, *enableLogging)
	switch mode := fi.Mode(); {
	case mode.IsDir():
		err := readFromDirectory(*inputFile, *keyword, sc)
		if err != nil {
			log.Fatal(logKey, "could not read from directory", "error", err)
		}
	case mode.IsRegular():
		err := readFromFile(*inputFile, *keyword, sc)
		if err != nil {
			log.Fatal(logKey, "could not read from file", "error", err)
		}
	}

	var buf bytes.Buffer
	header := fmt.Sprintf("search for keyword %s\nurl,found,context\n", *keyword)
	_, err = buf.WriteString(header)
	if err != nil {
		log.Error(logKey, "buffer could not write initial string")
	}

	res := sc.GetResults()
	sort.Sort(res)
	for _, r := range res {
		line := fmt.Sprintf("%s, %v, %v\n", r.URL, r.Found, r.Context)
		_, err = buf.WriteString(line)
		if err != nil {
			log.Fatal(logKey, "couldn't write string", "message", line)
		}
	}

	err = ioutil.WriteFile(*outFile, buf.Bytes(), 0644)
	if err != nil {
		log.Fatal(logKey, "couldn't write file", "error", err)
	}
}
