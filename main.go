package main

import (
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/pprof"
	"strconv"
	"strings"
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

	prt, _ := strconv.Atoi(*port)
	Acceptor(prt)

	/*
		err := fasthttp.ListenAndServe(":"+*port, handler)
		if err != nil {
			log.Fatal(err)
		}
	*/
}

var GET = []byte("GET")

func myHandler(ctx *Request) error {
	meth := ctx.Method
	path := ctx.Path
	logf("Method: %s Path: %s, args: %s", meth, path, ctx.Args)
	if !strings.HasPrefix(path, "/accounts/") {
		if path == "/start_profile" {
			fileName := ctx.GetArg("file")
			if fileName == "" {
				fileName = "cpu.out"
			}
			f, err := os.Create(fileName)
			if err != nil {
				log.Fatal("could not create CPU profile: ", err)
			}
			if err := pprof.StartCPUProfile(f); err != nil {
				log.Fatal("could not start CPU profile: ", err)
			}
		} else if path == "/stop_profile" {
			pprof.StopCPUProfile()
		} else if path == "/test" {
			ctx.SetStatusCode(200)
			ctx.SetBody([]byte("{}"))
			return nil
		}
		log.Printf("unknown path %s", meth)
		ctx.SetStatusCode(400)
		return nil
	}
	switch meth {
	case "GET":
		getHandler(ctx, path[10:])
	case "POST":
		postHandler(ctx, path[10:])
	default:
		log.Printf("unknown method %s", meth)
		ctx.SetStatusCode(400)
	}
	return nil
}

var logf = func(string, ...interface{}) {}

//var logf = log.Printf

/*
func myHandler(req *Request) error {
	req.SetStatusCode(200)
	return nil
}
*/
