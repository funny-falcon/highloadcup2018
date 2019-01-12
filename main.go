package main

import (
	"flag"
	"log"
)

var datazip = flag.String("data", "/tmp/data/data.zip", "data file")
var options = flag.String("opts", "/tmp/data/options.txt", "options file")

func main() {
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	flag.Parse()
	Load()
}
