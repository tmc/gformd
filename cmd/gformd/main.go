// Commmand gformd proxies a google form and injects Google Tag Manager tags.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"

	"github.com/ericchiang/css"
	"golang.org/x/net/html"
)

var (
	flagVerbose       = flag.Bool("v", false, "If true, enable verbose output")
	flagGTMTag        = flag.String("gtm", "GTM-XXXXXX", "Google Tag Manager Tag to inject")
	flagDefaultFormID = flag.String("form-id", "1FAIpQLSc1NjARplvfxfsdfasdfasdfadf_8ZrV3fdsfsdfg8wbe_LLg", "Default Google form id")
)

func main() {
	flag.Parse()

	p := &proxy{}
	handler := http.NewServeMux()
	handler.Handle("/", p)
	handler.HandleFunc("/gstatic/", p.static)
	handler.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))

	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

type proxy struct{}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	gtmTag := *flagGTMTag
	parts := strings.Split(r.URL.Path[1:], "/")
	formID := parts[0]
	if formID == "" {
		formID = *flagDefaultFormID
	}
	if len(parts) > 1 {
		gtmTag = parts[1]
	}

	url := fmt.Sprintf("https://docs.google.com/forms/d/e/%s/viewform", formID)
	fmt.Println(url)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
	defer resp.Body.Close()
	err = injectGTM(gtmTag, resp.Body, w)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
}

func (p *proxy) static(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/gstatic/"):]
	fmt.Println("path:", path)
	url := fmt.Sprintf("https://www.gstatic.com/%s", path)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
	defer resp.Body.Close()
	io.Copy(w, resp.Body)
}

func injectGTM(gtm string, body io.Reader, w io.Writer) error {
	buf, _ := ioutil.ReadAll(body)
	buf = bytes.ReplaceAll(buf, []byte("https://www.gstatic.com/"), []byte(`/gstatic/`))
	node, err := html.Parse(bytes.NewReader(buf))
	if err != nil {
		return err
	}

	findAndWrite := func(selector string) {
		sel, err := css.Compile(selector)
		if err != nil {
			fmt.Fprintln(w, "err:", err)
			return
		}
		for _, ele := range sel.Select(node) {
			html.Render(w, ele)
		}
	}
	w.Write([]byte(`<html><head>`))
	findAndWrite("head")
	gtmTag := fmt.Sprintf(`<script>(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'gtm.start':
new Date().getTime(),event:'gtm.js'});var f=d.getElementsByTagName(s)[0],
j=d.createElement(s),dl=l!='dataLayer'?'&l='+l:'';j.async=true;j.src=
'https://www.googletagmanager.com/gtm.js?id='+i+dl;f.parentNode.insertBefore(j,f);
})(window,document,'script','dataLayer','%s');</script>`, gtm)
	w.Write([]byte(gtmTag))
	w.Write([]byte(`<style>
.freebirdFormviewerViewCenteredContent { margin-top: 9em; }
.freebirdCustomFont { font-family: "Nunito Sans",sans-serif }
.freebirdFormviewerViewNavigationPasswordWarning, .freebirdFormviewerViewFooterDisclaimer, .freebirdFormviewerViewFeedbackSubmitFeedbackButton, .freebirdFormviewerViewFooterImageContainer { visibility: hidden; }
</style>
`))
	w.Write([]byte(`</head>`))

	w.Write([]byte(`<body>`))
	findAndWrite("body")
	w.Write([]byte(fmt.Sprintf(`<noscript><iframe src="https://www.googletagmanager.com/ns.html?id=%s"
height="0" width="0" style="display:none;visibility:hidden"></iframe></noscript>`, gtm)))
	w.Write([]byte(`</body></html>`))
	return nil
}
