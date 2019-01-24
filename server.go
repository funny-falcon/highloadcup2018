// +build linux
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

const epollQueue = 4000

var epollfd int
var readChan = make(chan File, 128)

func Acceptor(port int) {
	runtime.LockOSThread()

	var err error
	epollfd, err = syscall.EpollCreate(10000)
	if err != nil {
		log.Fatal(err)
	}
	//go Epoller()
	ncpu := runtime.NumCPU()
	fmt.Print("numcpu ", ncpu)
	if ncpu < 4 {
		ncpu = 4
	} else if ncpu > 32 {
		ncpu = 32
	}
	ncpu += 1 //ncpu / 4
	for i := 0; i < ncpu; i++ {
		go EpollHttp()
		//go HTTPHandler()
	}

	sock, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		log.Fatal(err)
	}

	if err = syscall.SetsockoptInt(sock, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		log.Fatal(err)
	}

	err = syscall.Bind(sock, &syscall.SockaddrInet4{Port: port})
	if err != nil {
		log.Fatal(err)
	}

	err = syscall.Listen(sock, epollQueue)
	if err != nil {
		log.Fatal(err)
	}

	for {
		connfd, _, err := syscall.Accept(sock)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			log.Fatal(err)
		}

		if err = syscall.SetsockoptInt(connfd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1); err != nil {
			log.Fatal(err)
		}

		File(connfd).addToEpoll(true)
	}
}

func Epoller() {
	runtime.LockOSThread()
	events := make([]syscall.EpollEvent, epollQueue)
	for {
		nevents, err := syscall.EpollWait(epollfd, events, -1)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			log.Fatal(err)
		}
		for _, event := range events[:nevents] {
			switch {
			case event.Events&(syscall.EPOLLHUP|syscall.EPOLLRDHUP) != 0:
				syscall.Close(int(event.Fd))
			case event.Events&syscall.EPOLLIN != 0:
				readChan <- File(event.Fd)
			default:
				log.Fatalf("Unknown epoll event %x", event.Events)
			}
		}
	}
}

func EpollHttp() {
	runtime.LockOSThread()
	var events [1]syscall.EpollEvent
	var req Request
	for {
		nevents, err := syscall.EpollWait(epollfd, events[:], -1)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			log.Fatal(err)
		}
		if nevents == 1 {
			event := events[0]
			ok := false
			switch {
			case event.Events&(syscall.EPOLLHUP|syscall.EPOLLRDHUP) != 0:
			case event.Events&syscall.EPOLLIN != 0:
				ok = HTTPHandleFd(File(event.Fd), &req)
			default:
				log.Fatalf("Unknown epoll event %x", event.Events)
			}
			if !ok {
				File(event.Fd).Close()
			} else {
				File(event.Fd).addToEpoll(false)
			}
		}
	}
}

func HTTPHandler() {
	//runtime.LockOSThread()
	var req Request
	for fd := range readChan {
		HTTPHandleFd(fd, &req)
	}
}

func HTTPHandleFd(fd File, req *Request) bool {
	*req = Request{}
	req.File = fd
	err := req.Parse()
	if err != nil {
		log.Print(err)
		return false
	}
	err = myHandler(req)
	if err != nil {
		if !req.Written {
			req.SetStatusCode(500)
		}
		log.Print(err)
	}
	if !req.Written {
		req.SetBody(nil)
	}
	if err != nil || req.Err != nil {
		return false
	}
	return true
}

type File int

func (f File) addToEpoll(add bool) {
	var ev syscall.EpollEvent
	ev.Events = syscall.EPOLLRDHUP | syscall.EPOLLONESHOT | syscall.EPOLLIN
	ev.Fd = int32(f)
	kind := syscall.EPOLL_CTL_MOD
	if add {
		kind = syscall.EPOLL_CTL_ADD
	}
	if err := syscall.EpollCtl(epollfd, kind, int(f), &ev); err != nil {
		log.Fatal(err)
	}
}

func (f File) Read(b []byte) (n int, err error) {
repeat:
	n, err = syscall.Read(int(f), b)
	if err != nil {
		if err == syscall.EINTR {
			goto repeat
		}
		n = 0
	} else if err == nil && n == 0 {
		err = io.EOF
	}
	return
}

func (f File) Write(b []byte) (n int, err error) {
	for n < len(b) {
		var nn int
		nn, err = syscall.Write(int(f), b[n:])
		if err != nil {
			nn = 0
		}
		n += nn
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			break
		}
	}
	return
}

func (f File) Writev(bb [][]byte) (n int, err error) {
	if len(bb) == 1 {
		return f.Write(bb[0])
	}
	iovecs := make([]syscall.Iovec, len(bb))
	totallen := 0
	for i, b := range bb {
		iovec := syscall.Iovec{Base: &b[0]}
		iovec.SetLen(len(b))
		iovecs[i] = iovec
		totallen += len(b)
	}
	var nn uintptr
	var serr syscall.Errno
repeat:
	nn, _, serr = syscall.Syscall(syscall.SYS_WRITEV,
		uintptr(f),
		uintptr(unsafe.Pointer(&iovecs[0])),
		uintptr(len(iovecs)))
	if serr == 0 {
		err = nil
	} else if serr == syscall.EINTR {
		goto repeat
	} else {
		err = serr
	}
	if err != nil {
		return
	} else {
		n = int(nn)
	}
	if n < totallen {
		k := n
		for k > len(bb[0]) {
			k -= len(bb[0])
			bb = bb[1:]
		}
		bb[0] = bb[0][k:]
		var nn int
		nn, err = f.Writev(bb)
		if err != nil {
			return
		}
		n += nn
	}
	return
}

func (f File) Close() error {
	return syscall.Close(int(f))
}

type Request struct {
	File          File
	BufBuf        [8192]byte
	Filled        int
	LastLine      int
	ContentLength int
	EOF           bool
	Method        string
	Path          string
	Args          []kv
	Body          []byte

	Status  int
	Err     error
	Written bool
}

type kv struct {
	k, v string
}

func (r *Request) Parse() error {
	for {
		nextLine := bytes.Index(r.BufBuf[r.LastLine:r.Filled], []byte("\r\n"))
		if nextLine == -1 {
			if err := r.read(len(r.BufBuf)); err != nil {
				return err
			}
			continue
		}
		line := r.BufBuf[r.LastLine : r.LastLine+nextLine]
		if r.LastLine <= 2 {
			if len(line) == 0 {
				r.LastLine = 2
				continue
			} else if line[0] == '\n' {
				line = line[1:]
			}
			methix := bytes.IndexByte(line, ' ')
			if methix == -1 {
				return fmt.Errorf("No space in first line: %q", string(line))
			}
			r.Method = b2s(r.BufBuf[:methix])

			uriix := methix + 1 + bytes.IndexByte(line[methix+1:], ' ')
			if uriix == methix {
				return errors.New("No uri end")
			}
			uri := line[methix+1 : uriix]

			queryix := bytes.IndexByte(uri, '?')
			if queryix == -1 {
				r.Path = b2s(uri)
			} else {
				r.Path = b2s(uri[:queryix])
				r.parseArgs(uri[queryix+1:])
			}
		} else if bytes.HasPrefix(line, []byte("Content-Length: ")) {
			r.ContentLength, _ = strconv.Atoi(string(line[16:]))
		}
		r.LastLine += nextLine + 2
		if len(line) == 0 {
			break
		}
	}

	for r.LastLine+r.ContentLength > r.Filled {
		if err := r.read(r.LastLine + r.ContentLength + 2); err != nil {
			return err
		}
	}
	r.Body = r.BufBuf[r.LastLine : r.LastLine+r.ContentLength]
	return nil
}

func (r *Request) SetStatusCode(cd int) {
	r.Status = cd
}

func (r *Request) SetBody(b []byte) {
	n := copy(r.BufBuf[:], "HTTP/1.1 ")
	switch r.Status {
	case 0, 200:
		n += copy(r.BufBuf[n:], "200 OK\r\n")
	case 201:
		n += copy(r.BufBuf[n:], "201 Created\r\n")
	case 202:
		n += copy(r.BufBuf[n:], "202 Accepted\r\n")
	case 400:
		n += copy(r.BufBuf[n:], "400 Bad Request\r\n")
	case 404:
		n += copy(r.BufBuf[n:], "404 Not Found\r\n")
	default:
		n += copy(r.BufBuf[n:], fmt.Sprintf("%d Some Code\r\n", r.Status))
	}
	n += copy(r.BufBuf[n:], "Server: fake-server v0.1\r\n")

	n += copy(r.BufBuf[n:], "Date: ")
	n += copy(r.BufBuf[n:], time.Now().UTC().Format(http.TimeFormat))
	n += copy(r.BufBuf[n:], "\r\n")

	n += copy(r.BufBuf[n:], "Connection: keep-alive\r\n")

	if len(b) > 0 {
		n += copy(r.BufBuf[n:], "Content-Type: application/json\r\n")
		n += copy(r.BufBuf[n:], "Content-Length: ")
		n += copy(r.BufBuf[n:], strconv.Itoa(len(b)))
		n += copy(r.BufBuf[n:], "\r\n")
	} else {
		n += copy(r.BufBuf[n:], "Content-Length: 0\r\n")
	}

	n += copy(r.BufBuf[n:], "\r\n")

	var err error
	if len(b) > 0 {
		_, err = r.File.Writev([][]byte{r.BufBuf[:n], b})
	} else {
		_, err = r.File.Write(r.BufBuf[:n])
	}
	if err != nil {
		log.Print(err)
		r.Err = err
	}
	r.Written = true
}

func (r *Request) parseArgs(args []byte) {
	for len(args) > 0 {
		andix := bytes.IndexByte(args, '&')
		var arg []byte
		if andix == -1 {
			arg = args
			args = nil
		} else {
			arg = args[:andix]
			args = args[andix+1:]
		}

		var kv kv

		assignix := bytes.IndexByte(arg, '=')
		if assignix == -1 {
			kv.k = b2s(arg)
		} else {
			kv.k = b2s(arg[:assignix])
			v := arg[assignix+1:]
			i := 0
			for j := 0; j < len(v); j++ {
				c := v[j]
				if c == '%' && j+2 < len(v) {
					b := c2d(v[j+1])<<4 | c2d(v[j+2])
					v[i] = b
					j += 2
				} else if c == '+' {
					v[i] = ' '
				} else {
					v[i] = c
				}
				i++
			}
			kv.v = b2s(v[:i])
		}
		r.Args = append(r.Args, kv)
	}
}

func (r *Request) GetArg(k string) string {
	for _, kv := range r.Args {
		if kv.k == k {
			return kv.v
		}
	}
	return ""
}

func (r *Request) VisitArgs(f func(k, v string)) {
	for _, kv := range r.Args {
		f(kv.k, kv.v)
	}
}

func c2d(c byte) byte {
	if '0' <= c && c <= '9' {
		return c - '0'
	}
	return (c &^ 0x20) - 'A' + 10
}

func (r *Request) read(lim int) error {
	n, err := r.File.Read(r.BufBuf[r.Filled:lim])
	r.Filled += n
	return err
}

func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
