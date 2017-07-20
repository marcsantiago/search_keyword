package main

import (
	"fmt"

	"./search"
)

func main() {
	s, err := search.NormalizeURL("facebook.com/")
	if err != nil {
		panic(err)
	}
	fmt.Println(s)
}
