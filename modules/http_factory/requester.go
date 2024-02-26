package http_factory

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/time/rate"
	"io/ioutil"
	"net"
	url2 "net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	StartTime        = time.Now()
	SendOnCloseError interface{}
)

type ReportRecord struct {
	Cost       time.Duration
	Code       string
	Error      string
	ReadBytes  int64
	WriteBytes int64
}

var RecordPool = sync.Pool{
	New: func() interface{} { return new(ReportRecord) },
}

func init() {
	// Honoring env GOMAXPROCS
	_, _ = maxprocs.Set()
	defer func() {
		SendOnCloseError = recover()
	}()
	func() {
		cc := make(chan struct{}, 1)
		close(cc)
		cc <- struct{}{}
	}()
}

type MyConn struct {
	net.Conn
	r, w *int64
}

func NewMyConn(conn net.Conn, r, w *int64) (*MyConn, error) {
	myConn := &MyConn{Conn: conn, r: r, w: w}
	return myConn, nil
}

func (c *MyConn) Read(b []byte) (n int, err error) {
	sz, err := c.Conn.Read(b)

	if err == nil {
		atomic.AddInt64(c.r, int64(sz))
	}
	return sz, err
}

func (c *MyConn) Write(b []byte) (n int, err error) {
	sz, err := c.Conn.Write(b)

	if err == nil {
		atomic.AddInt64(c.w, int64(sz))
	}
	return sz, err
}

func ThroughputInterceptorDial(dial fasthttp.DialFunc, r *int64, w *int64) fasthttp.DialFunc {
	return func(addr string) (net.Conn, error) {
		conn, err := dial(addr)
		if err != nil {
			return nil, err
		}
		return NewMyConn(conn, r, w)
	}
}

type Requester struct {
	concurrency int
	reqRate     *rate.Limit
	requests    int64
	duration    time.Duration
	clientOpt   *ClientOpt
	httpClient  *fasthttp.HostClient
	httpHeader  *fasthttp.RequestHeader

	recordChan chan *ReportRecord
	closeOnce  sync.Once
	wg         sync.WaitGroup

	readBytes  int64
	writeBytes int64

	cancel func()
}

type ClientOpt struct {
	TestID    string
	Url       string
	Method    string
	Headers   []string
	BodyBytes []byte
	BodyFile  string

	CertPath string
	KeyPath  string
	Insecure bool

	MaxConns     int
	DoTimeout    time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	DialTimeout  time.Duration

	Socks5Proxy string
	ContentType string
	Host        string
}

func NewRequester(concurrency int, requests int64, duration time.Duration, reqRate *rate.Limit, clientOpt *ClientOpt) (*Requester, error) {
	maxResult := concurrency * 100
	if maxResult > 8192 {
		maxResult = 8192
	}
	r := &Requester{
		concurrency: concurrency,
		reqRate:     reqRate,
		requests:    requests,
		duration:    duration,
		clientOpt:   clientOpt,
		recordChan:  make(chan *ReportRecord, maxResult),
	}
	client, header, err := buildRequestClient(clientOpt, &r.readBytes, &r.writeBytes)
	if err != nil {
		return nil, err
	}
	r.httpClient = client
	r.httpHeader = header
	return r, nil
}

func addMissingPort(addr string, isTLS bool) string {
	n := strings.Index(addr, ":")
	if n >= 0 {
		return addr
	}
	port := 80
	if isTLS {
		port = 443
	}
	return net.JoinHostPort(addr, strconv.Itoa(port))
}

func buildTLSConfig(opt *ClientOpt) (*tls.Config, error) {
	var certs []tls.Certificate
	if opt.CertPath != "" && opt.KeyPath != "" {
		c, err := tls.LoadX509KeyPair(opt.CertPath, opt.KeyPath)
		if err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}
	return &tls.Config{
		InsecureSkipVerify: opt.Insecure,
		Certificates:       certs,
	}, nil
}

func buildRequestClient(opt *ClientOpt, r *int64, w *int64) (*fasthttp.HostClient, *fasthttp.RequestHeader, error) {
	u, err := url2.Parse(opt.Url)
	if err != nil {
		return nil, nil, err
	}
	httpClient := &fasthttp.HostClient{
		Addr:                          addMissingPort(u.Host, u.Scheme == "https"),
		IsTLS:                         u.Scheme == "https",
		Name:                          "plow",
		MaxConns:                      opt.MaxConns,
		ReadTimeout:                   opt.ReadTimeout,
		WriteTimeout:                  opt.WriteTimeout,
		DisableHeaderNamesNormalizing: true,
	}
	if opt.Socks5Proxy != "" {
		if !strings.Contains(opt.Socks5Proxy, "://") {
			opt.Socks5Proxy = "socks5://" + opt.Socks5Proxy
		}
		httpClient.Dial = fasthttpproxy.FasthttpSocksDialer(opt.Socks5Proxy)

	} else {
		httpClient.Dial = fasthttpproxy.FasthttpProxyHTTPDialerTimeout(opt.DialTimeout)
	}
	httpClient.Dial = ThroughputInterceptorDial(httpClient.Dial, r, w)

	tlsConfig, err := buildTLSConfig(opt)
	if err != nil {
		return nil, nil, err
	}
	httpClient.TLSConfig = tlsConfig

	var requestHeader fasthttp.RequestHeader
	if opt.ContentType != "" {
		requestHeader.SetContentType(opt.ContentType)
	}
	if opt.Host != "" {
		requestHeader.SetHost(opt.Host)
	} else {
		requestHeader.SetHost(u.Host)
	}
	requestHeader.SetMethod(opt.Method)
	requestHeader.SetRequestURI(u.RequestURI())
	for _, h := range opt.Headers {
		n := strings.SplitN(h, ":", 2)
		if len(n) != 2 {
			return nil, nil, fmt.Errorf("invalid header: %s", h)
		}
		requestHeader.Set(n[0], n[1])
	}

	return httpClient, &requestHeader, nil
}

func (r *Requester) Cancel() {
	r.cancel()
}

func (r *Requester) RecordChan() <-chan *ReportRecord {
	return r.recordChan
}

func (r *Requester) closeRecord() {
	r.closeOnce.Do(func() {
		close(r.recordChan)
	})
}

func (r *Requester) DoRequest(req *fasthttp.Request, resp *fasthttp.Response, rr *ReportRecord) {
	t1 := time.Since(StartTime)
	var err error
	if r.clientOpt.DoTimeout > 0 {
		err = r.httpClient.DoTimeout(req, resp, r.clientOpt.DoTimeout)
	} else {
		err = r.httpClient.Do(req, resp)
	}
	var code string

	if err != nil {
		rr.Cost = time.Since(StartTime) - t1
		rr.Code = ""
		rr.Error = err.Error()
		return
	}
	switch resp.StatusCode() / 100 {
	case 1:
		code = "1xx"
	case 2:
		code = "2xx"
	case 3:
		code = "3xx"
	case 4:
		code = "4xx"
	case 5:
		code = "5xx"
	}
	err = resp.BodyWriteTo(ioutil.Discard)
	if err != nil {
		rr.Cost = time.Since(StartTime) - t1
		rr.Code = ""
		rr.Error = err.Error()
		return
	}

	rr.Cost = time.Since(StartTime) - t1
	rr.Code = code
	rr.Error = ""
}

func (r *Requester) Run() {
	// handle ctrl-c
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	ctx, cancelFunc := context.WithCancel(context.Background())
	r.cancel = cancelFunc
	go func() {
		<-sigs
		r.closeRecord()
		cancelFunc()
	}()
	StartTime = time.Now()
	if r.duration > 0 {
		time.AfterFunc(r.duration, func() {
			r.closeRecord()
			cancelFunc()
		})
	}

	var limiter *rate.Limiter
	if r.reqRate != nil {
		limiter = rate.NewLimiter(*r.reqRate, 1)
	}

	semaphore := r.requests
	for i := 0; i < r.concurrency; i++ {
		r.wg.Add(1)
		go func() {
			defer func() {
				r.wg.Done()
				v := recover()
				if v != nil && v != SendOnCloseError {
					panic(v)
				}
			}()
			req := &fasthttp.Request{}
			resp := &fasthttp.Response{}
			r.httpHeader.CopyTo(&req.Header)
			if r.httpClient.IsTLS {
				req.URI().SetScheme("https")
				req.URI().SetHostBytes(req.Header.Host())
			}

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if limiter != nil {
					err := limiter.Wait(ctx)
					if err != nil {
						continue
					}
				}

				if r.requests > 0 && atomic.AddInt64(&semaphore, -1) < 0 {
					cancelFunc()
					return
				}

				if r.clientOpt.BodyFile != "" {
					file, err := os.Open(r.clientOpt.BodyFile)
					if err != nil {
						rr := RecordPool.Get().(*ReportRecord)
						rr.Cost = 0
						rr.Error = err.Error()
						rr.ReadBytes = atomic.LoadInt64(&r.readBytes)
						rr.WriteBytes = atomic.LoadInt64(&r.writeBytes)
						r.recordChan <- rr
						continue
					}
					req.SetBodyStream(file, -1)
				} else {
					req.SetBodyRaw(r.clientOpt.BodyBytes)
				}
				resp.Reset()
				rr := RecordPool.Get().(*ReportRecord)
				r.DoRequest(req, resp, rr)
				rr.ReadBytes = atomic.LoadInt64(&r.readBytes)
				rr.WriteBytes = atomic.LoadInt64(&r.writeBytes)
				r.recordChan <- rr
			}
		}()
	}

	r.wg.Wait()
	r.closeRecord()
}
