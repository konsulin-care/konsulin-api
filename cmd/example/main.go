package main

import "fmt"

// Version sets the default build version
var Version = "develop"

// Tag sets the default latest commit tag
var Tag = "0.0.1-rc"

func main() {
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Tag: %s\n", Tag)
}
