package main

import (
	"bytes"
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
var dumpload = flag.Bool("dumpload", false, "dumpload")

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
	meth := ctx.Method()
	path := ctx.Path()
	if !bytes.HasPrefix(path, []byte("/accounts/")) {
		ctx.SetStatusCode(400)
		return
	}
	logf("Method: %s Path: %s, args: %s", meth, path, ctx.QueryArgs())
	switch {
	case ctx.IsGet():
		getHandler(ctx, path[10:])
	case ctx.IsPost():
		postHandler(ctx, path[10:])
	}
}

func logf(format string, args ...interface{}) {
	//log.Printf(format, args...)
}
