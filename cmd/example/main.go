package main

import "fmt"

// Version sets the default build version
var Version = "develop"

// Tag sets the default latest commit tag
var Tag = "0.0.1-rc"

func main() {
	fmt.Println(fmt.Sprintf("Version: %s", Version))
	fmt.Println(fmt.Sprintf("Tag: %s", Tag))
}
