package main

import (
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/valyala/fasthttp"
)

//var datazip = flag.String("data", "/tmp/data/data.zip", "data file")
//var options = flag.String("opts", "/tmp/data/options.txt", "options file")
var path = flag.String("path", "/tmp/data/", "data path")
var port = flag.String("port", "80", "port to listen")
var onlyload = flag.Bool("onlyload", false, "only load")
var memprofile = flag.String("memprofile", "", "memprofile")

func main() {
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	flag.Parse()

	go http.ListenAndServe("localhost:6065", nil)

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
