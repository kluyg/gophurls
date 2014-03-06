package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"github.com/PuerkitoBio/goquery"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
)

var httpAddr = flag.String("http", ":7000", "HTTP service address")

var data struct {
	URLs []Link
	sync.RWMutex
}

var homeTmpl *template.Template

type Link struct {
	URL   string
	Title string
}

func init() {
	// Set up the HTTP handler in init (not main) so we can test it. (This main
	// doesn't run when testing.)
	http.HandleFunc("/", home)
	http.HandleFunc("/links", links)
	http.HandleFunc("/peers", peersHandler)
	homeTmpl = template.Must(template.New("home").Parse(`<h1>GophURLs</h1>
<h2>Links</h2>
<ol>
  {{range .URLs}}<li><a href="{{.URL}}">{{.Title}}</a></li>{{end}}
</ol>`))
}

func main() {
	flag.Parse()
	if err := http.ListenAndServe(*httpAddr, nil); err != nil {
		log.Fatal(err)
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	data.RLock()
	defer data.RUnlock()
	homeTmpl.Execute(w, data)
}

func links(w http.ResponseWriter, r *http.Request) {
	var url Link
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("links - read req body", err)
		return
	}
	err = json.Unmarshal(bytes, &url)
	if err != nil {
		log.Println("links - unmarshal json", err)
		return
	}

	go addURL(url)
}

func peersHandler(w http.ResponseWriter, r *http.Request) {
	var list []string
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("peers - read req body", err)
		return
	}
	err = json.Unmarshal(bytes, &list)
	if err != nil {
		log.Println("peers - unmarshal json", err)
		return
	}

	peersLock.Lock()
	defer peersLock.Unlock()

	for _, v := range list {
		peers[v] = struct{}{}
	}
}

func check(url string) bool {
	for _, v := range data.URLs {
		if v.URL == url {
			return false
		}
	}
	return true
}

const CONTENT_TYPE = "text/html"

func addURL(link Link) {
	data.Lock()
	defer data.Unlock()

	if !check(link.URL) {
		log.Println("addURL - already have the link", link.URL)
		return
	}

	if link.Title != "" {
		data.URLs = append(data.URLs, link)
		go shareLink(link)
		return
	}

	resp, err := http.Get(link.URL)
	if err != nil {
		log.Println("addURL - getting the link", err)
		return
	}
	contentType := resp.Header.Get("Content-Type")[:len(CONTENT_TYPE)]
	if contentType != CONTENT_TYPE {
		log.Println("addURL - trying to submit nasty stuff", contentType)
		return
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.Println("trying to submit nasty stuff", contentType)
		return
	}
	link.Title = doc.Find("title").Text()
	data.URLs = append(data.URLs, link)

	go shareLink(link)
}

func shareLink(link Link) {
	peersLock.RLock()
	defer peersLock.RUnlock()

	arr, err := json.Marshal(link)
	if err != nil {
		log.Println("shareLink - can't marshal link", err)
		return
	}
	arr = append(arr, '\n')
	for v := range peers {
		go http.Post("http://"+v+"/links", "application/json", bytes.NewReader(arr))
	}
}
