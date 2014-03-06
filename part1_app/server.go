package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"text/template"
)

var httpAddr = flag.String("http", ":7000", "HTTP service address")
var verbose = flag.Bool("v", false, "show verbose output")

type link struct {
	URL string
}

var (
	links      []*link
	linksMutex sync.RWMutex
)

func init() {
	// Set up the HTTP handler in init (not main) so we can test it. (This main
	// doesn't run when testing.)
	http.HandleFunc("/links", addLink)
	http.HandleFunc("/", home)
}

func main() {
	flag.Parse()
	if err := http.ListenAndServe(*httpAddr, nil); err != nil {
		log.Fatal(err)
	}
}

var homeTmpl = template.Must(template.New("Home").Parse(`
<h1>GophURLs <img width=28 height=38 src="http://golang.org/doc/gopher/frontpage.png"></h1>
<p>Submit a link: <tt>curl -X POST -d '{"URL":"http://example.com"}' http://localhost:7000/links</tt></p>
<h2>Links ({{len .}})</h2>
<ol>
{{/* Iterate over the data we passed to the template (links). */}}
{{range .}}
  <li><a href="{{.URL}}">{{.URL}}</a></li>
{{end}}
</ol>
`))

func home(w http.ResponseWriter, r *http.Request) {
	// Lock links for reading.
	linksMutex.RLock()
	defer linksMutex.RUnlock()

	// Render the template.
	homeTmpl.Execute(w, links)
}

func addLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "bad method", http.StatusBadRequest)
		return
	}

	var link *link
	err := json.NewDecoder(r.Body).Decode(&link)
	if err != nil {
		http.Error(w, fmt.Sprintf("bad JSON: %s", err), http.StatusBadRequest)
		return
	}

	// Validate the URL.
	if link.URL == "" {
		http.Error(w, "no url", http.StatusBadRequest)
		return
	}
	url, err := url.Parse(link.URL)
	if err != nil {
		http.Error(w, "bad url", http.StatusBadRequest)
		return
	}
	if !url.IsAbs() {
		http.Error(w, "url must be absolute", http.StatusBadRequest)
		return
	}

	// Lock links for writing and add the new link.
	linksMutex.Lock()
	defer linksMutex.Unlock()
	links = append(links, link)
}