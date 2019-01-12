package main

import (
	"flag"
	"log"

	"github.com/valyala/fasthttp"
)

var datazip = flag.String("data", "/tmp/data/data.zip", "data file")
var options = flag.String("opts", "/tmp/data/options.txt", "options file")
var port = flag.String("port", "80", "port to listen")
var onlyload = flag.Bool("onlyload", false, "only load")

func main() {
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	flag.Parse()
	Load()

	if *onlyload {
		return
	}

	err := fasthttp.ListenAndServe(":"+*port, handler)
	if err != nil {
		log.Fatal(err)
	}
}

var GET = []byte("GET")

func handler(ctx *fasthttp.RequestCtx) {
	switch {
	case ctx.IsGet():
		getHandler(ctx)
	}
}
