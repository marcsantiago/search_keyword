package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"./search"

	log "github.com/sirupsen/logrus"
)

// this particular main function is written in such a way to satisfy
// the questions requirement, however the package search was written to
// be more generic
func main() {
	inputFile := flag.String("in", "", "the input file path containing the list of urls")
	outFile := flag.String("out", "", "output file path")
	keyword := flag.String("keyword", "", "keyword to search for")
	limit := flag.Int("limit", 20, "set the limit of goroutines to spin up")
	flag.Parse()

	if *inputFile == "" {
		flag.PrintDefaults()
		log.Fatal("input file path cannot be empty")
	}

	if *outFile == "" {
		flag.PrintDefaults()
		log.Fatal("out file path cannot be empty")
	}

	if *keyword == "" {
		flag.PrintDefaults()
		log.Fatal("keyword cannot be empty")
	}

	file, err := os.Open(*inputFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	var wg sync.WaitGroup
	sc := search.NewScanner(*limit, true)

	// reading file line by line, no need to store whole file in mem
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		wg.Add(1)
		line := scanner.Text()
		// for this particular example urls are stored in a file like so
		// 1,"facebook.com/",9616487,1688316928,9.54,9.34
		// so I'm going to parse that file to get the url
		go func(line string, wg *sync.WaitGroup, sc *search.Scanner) {
			defer wg.Done()
			parts := strings.Split(line, ",")
			URL := strings.Replace(parts[1], "\"", "", -1)
			err := sc.Search(URL, *keyword)
			if err != nil {
				log.Warningf("%s had and issue %v\n", URL, err)
			}
		}(line, &wg, sc)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	wg.Wait()

	var buf bytes.Buffer
	header := fmt.Sprintf("search for keyword %s\nurl,found\n", *keyword)
	_, err = buf.WriteString(header)
	if err != nil {
		log.Warning("Buffer could not write initial string")
	}

	// two ways of getting the data..directly from the map
	for key, val := range sc.WasFound {
		line := fmt.Sprintf("%s, %v\n", key, val)
		_, err = buf.WriteString(line)
		if err != nil {
			log.Warningf("couldn't write string %s", line)
		}
	}
	err = ioutil.WriteFile(*outFile, buf.Bytes(), 0644)
	if err != nil {
		log.Fatal(err)
	}

	// OR more raw...or however you see fit

	// reader, err := sc.MapToIOReaderWriter()
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// b, err := json.MarshalIndent(reader, "", " ")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// err = ioutil.WriteFile(*outFile, b, 0644)
	// if err != nil {
	// 	log.Fatal(err)
	// }

}
