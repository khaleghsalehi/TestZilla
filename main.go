package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"strings"
	"testzilla/core"
	http2 "testzilla/modules/http_factory"
	"time"

	"golang.org/x/time/rate"

	"gopkg.in/alecthomas/kingpin.v3-unstable"
)

var (
	concurrency = kingpin.Flag("concurrency", "Number of connections to run concurrently").Short('c').Default("1").Int()
	reqRate     = rateFlag(kingpin.Flag("rate", "Number of requests per time unit, examples: --rate 50 --rate 10/ms").Default("infinity"))
	requests    = kingpin.Flag("requests", "Number of requests to run").Short('n').Default("-1").Int64()
	duration    = kingpin.Flag("duration", "Duration of test, examples: -d 10s -d 3m").Short('d').PlaceHolder("DURATION").Duration()
	interval    = kingpin.Flag("interval", "Print snapshot result every interval, use 0 to print once at the end").Short('i').Default("200ms").Duration()
	seconds     = kingpin.Flag("seconds", "Use seconds as time unit to print").Bool()
	jsonFormat  = kingpin.Flag("json", "Print snapshot result as JSON").Bool()

	body      = kingpin.Flag("body", "HTTP request body, if body starts with '@' the rest will be considered a file's path from which to read the actual body content").Short('b').String()
	stream    = kingpin.Flag("stream", "Specify whether to stream file specified by '--body @file' using chunked encoding or to read into memory").Default("false").Bool()
	methodSet = false
	method    = kingpin.Flag("method", "HTTP method").Action(func(_ *kingpin.ParseElement, _ *kingpin.ParseContext) error {
		methodSet = true
		return nil
	}).Default("GET").Short('m').String()
	headers     = kingpin.Flag("header", "Custom HTTP headers").Short('H').PlaceHolder("K:V").Strings()
	host        = kingpin.Flag("host", "Host header").String()
	contentType = kingpin.Flag("content", "Content-Type header").Short('T').String()
	cert        = kingpin.Flag("cert", "Path to the client's TLS Certificate").ExistingFile()
	key         = kingpin.Flag("key", "Path to the client's TLS Certificate Private Key").ExistingFile()
	insecure    = kingpin.Flag("insecure", "Controls whether a client verifies the server's certificate chain and host name").Short('k').Bool()

	timeout         = kingpin.Flag("timeout", "Timeout for each http_factory request").PlaceHolder("DURATION").Duration()
	dialTimeout     = kingpin.Flag("dial-timeout", "Timeout for dial addr").PlaceHolder("DURATION").Duration()
	reqWriteTimeout = kingpin.Flag("req-timeout", "Timeout for full request writing").PlaceHolder("DURATION").Duration()
	respReadTimeout = kingpin.Flag("resp-timeout", "Timeout for full response reading").PlaceHolder("DURATION").Duration()
	socks5          = kingpin.Flag("socks5", "Socks5 proxy").PlaceHolder("ip:port").String()

	autoOpenBrowser = kingpin.Flag("auto-open-browser", "Specify whether auto open browser to show web charts").Bool()
	clean           = kingpin.Flag("clean", "Clean the histogram bar once its finished. Default is true").Default("true").NegatableBool()
	summary         = kingpin.Flag("summary", "Only print the summary without realtime reports").Default("false").Bool()
	pprofAddr       = kingpin.Flag("pprof", "Enable pprof at special address").Hidden().String()
	url             = kingpin.Arg("url", "Request url").Required().String()
)

// dynamically set by GoReleaser

func errAndExit(msg string) {
	fmt.Fprintln(os.Stderr, "testzilla: "+msg)
	os.Exit(1)
}

var CompactUsageTemplate = `{{define "FormatCommand" -}}
{{if .FlagSummary}} {{.FlagSummary}}{{end -}}
{{range .Args}} {{if not .Required}}[{{end}}<{{.Name}}>{{if .Value|IsCumulative}} ...{{end}}{{if not .Required}}]{{end}}{{end -}}
{{end -}}

{{define "FormatCommandList" -}}
{{range . -}}
{{if not .Hidden -}}
{{.Depth|Indent}}{{.Name}}{{if .Default}}*{{end}}{{template "FormatCommand" .}}
{{end -}}
{{template "FormatCommandList" .Commands -}}
{{end -}}
{{end -}}

{{define "FormatUsage" -}}
{{template "FormatCommand" .}}{{if .Commands}} <command> [<args> ...]{{end}}
{{if .Help}}
{{.Help|Wrap 0 -}}
{{end -}}

{{end -}}

{{if .Context.SelectedCommand -}}
{{T "usage:"}} {{.App.Name}} {{template "FormatUsage" .Context.SelectedCommand}}
{{else -}}
{{T "usage:"}} {{.App.Name}}{{template "FormatUsage" .App}}
{{end -}}
Examples:

  testzilla http://127.0.0.1:8080/ -c 20 -n 100000
  testzilla https://exmaple.com/post -c 20 -d 5m --body @file.json -T 'application/json' -m POST

{{if .Context.Flags -}}
{{T "Flags:"}}
{{.Context.Flags|FlagsToTwoColumns|FormatTwoColumns}}
  Flags default values also read from env PLOW_SOME_FLAG, such as PLOW_TIMEOUT=5s equals to --timeout=5s

{{end -}}
{{if .Context.Args -}}
{{T "Args:"}}
{{.Context.Args|ArgsToTwoColumns|FormatTwoColumns}}
{{end -}}
{{if .Context.SelectedCommand -}}
{{if .Context.SelectedCommand.Commands -}}
{{T "Commands:"}}
  {{.Context.SelectedCommand}}
{{.Context.SelectedCommand.Commands|CommandsToTwoColumns|FormatTwoColumns}}
{{end -}}
{{else if .App.Commands -}}
{{T "Commands:"}}
{{.App.Commands|CommandsToTwoColumns|FormatTwoColumns}}
{{end -}}
`

type rateFlagValue struct {
	infinity bool
	limit    rate.Limit
	v        string
}

func (f *rateFlagValue) Set(v string) error {
	if v == "infinity" {
		f.infinity = true
		return nil
	}

	retErr := fmt.Errorf("--rate format %q doesn't match the \"freq/duration\" (i.e. 50/1s)", v)
	ps := strings.SplitN(v, "/", 2)
	switch len(ps) {
	case 1:
		ps = append(ps, "1s")
	case 0:
		return retErr
	}

	freq, err := strconv.Atoi(ps[0])
	if err != nil {
		return retErr
	}
	if freq == 0 {
		f.infinity = true
		return nil
	}

	switch ps[1] {
	case "ns", "us", "µs", "ms", "s", "m", "h":
		ps[1] = "1" + ps[1]
	}

	per, err := time.ParseDuration(ps[1])
	if err != nil {
		return retErr
	}

	f.limit = rate.Limit(float64(freq) / per.Seconds())
	f.v = v
	return nil
}

func (f *rateFlagValue) Limit() *rate.Limit {
	if f.infinity {
		return nil
	}
	return &f.limit
}

func (f *rateFlagValue) String() string {
	return f.v
}

func rateFlag(c *kingpin.Clause) (target *rateFlagValue) {
	target = new(rateFlagValue)
	c.SetValue(target)
	return
}

func main() {
	fmt.Fprintln(os.Stderr, "\U0001F996 TestZilla, Version "+core.TestzillaVersion)
	kingpin.UsageTemplate(CompactUsageTemplate).
		Version(core.TestzillaVersion).
		Author("KhaleghSalehi@Gmail.com").
		Resolver(kingpin.PrefixedEnvarResolver("TESTZILLA_", ";")).
		Help = `A high-performance HTTP benchmarking tool with real-time statistical`
	kingpin.Parse()

	if *requests >= 0 && *requests < int64(*concurrency) {
		errAndExit("requests must greater than or equal concurrency")
		return
	}
	if (*cert != "" && *key == "") || (*cert == "" && *key != "") {
		errAndExit("must specify cert and key at the same time")
		return
	}

	if *pprofAddr != "" {
		go http.ListenAndServe(*pprofAddr, nil)
	}

	var err error
	var bodyBytes []byte
	var bodyFile string

	if *body != "" {
		if strings.HasPrefix(*body, "@") {
			fileName := (*body)[1:]
			if _, err = os.Stat(fileName); err != nil {
				errAndExit(err.Error())
				return
			}
			if *stream {
				bodyFile = fileName
			} else {
				bodyBytes, err = os.ReadFile(fileName)
				if err != nil {
					errAndExit(err.Error())
					return
				}
			}
		} else {
			bodyBytes = []byte(*body)
		}

		if !methodSet {
			*method = "POST"
		}
	}

	clientOpt := http2.ClientOpt{
		Url:       *url,
		Method:    *method,
		Headers:   *headers,
		BodyBytes: bodyBytes,
		BodyFile:  bodyFile,

		CertPath: *cert,
		KeyPath:  *key,
		Insecure: *insecure,

		MaxConns:     *concurrency,
		DoTimeout:    *timeout,
		ReadTimeout:  *respReadTimeout,
		WriteTimeout: *reqWriteTimeout,
		DialTimeout:  *dialTimeout,

		Socks5Proxy: *socks5,
		ContentType: *contentType,
		Host:        *host,
	}

	requester, err := http2.NewRequester(*concurrency, *requests, *duration, reqRate.Limit(), &clientOpt)
	if err != nil {
		errAndExit(err.Error())
		return
	}

	// description
	fmt.Fprintln(os.Stderr, "\U0001F996 TestZilla, Version "+core.TestzillaVersion)
	var desc string
	desc = fmt.Sprintf("Benchmarking %s", *url)
	if *requests > 0 {
		desc += fmt.Sprintf(" with %d request(s)", *requests)
	}
	if *duration > 0 {
		desc += fmt.Sprintf(" for %s", duration.String())
	}
	desc += fmt.Sprintf(" using %d connection(s).", *concurrency)
	fmt.Fprintln(os.Stderr, desc)

	fmt.Fprintln(os.Stderr, "")

	// do request
	go requester.Run()

	// metrics collection
	report := core.NewStreamReport()
	go report.Collect(requester.RecordChan())

	// terminal printer
	printer := core.NewPrinter(*requests, *duration, !*clean, *summary)
	printer.PrintLoop(report.Snapshot, *interval, *seconds, *jsonFormat, report.Done())
}
