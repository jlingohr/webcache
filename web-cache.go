package main

import (
	"./webcache"
	"bytes"
	"errors"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	wc          webcache.Cache
	dc          *webcache.DiskCache
	invertedMap *webcache.InvertedIndex
	client      *http.Client
	ipPort1     *net.TCPAddr
	ipPort2     *net.TCPAddr
)

const GET = "GET"
const CONTENT_TYPE = "Content-Type"
const HTML_TYPE = "text/html"
const LRU = "LRU"
const LFU = "LFU"
const HTTP_PREFIX = "http://"
const CUSTOM_URL_PREFIX = "http://name_of_server/"
const CACHE_ROOT = "cache"

func main() {
	args := os.Args[1:]

	if len(args) != 5 {
		fmt.Print("Usage: web-cache.go [ip1:port1] [ip2:port2] [replacement_policy] [cache_size] [expiration_time]")
		return
	}

	var err error
	ipPort1, err = getAddress(args[0])
	if err != nil {
		log.Fatal(err)
	}

	ipPort2, err = getAddress(args[1])
	if err != nil {
		log.Fatal(err)
	}

	replacementPolicy := args[2]
	cacheSize, err := strconv.ParseUint(args[3], 10, 32)
	if err != nil {
		log.Fatal("Invalid parameter [cache_size]")
	}
	expirationTime, err := strconv.Atoi(args[4])
	if err != nil || expirationTime < 0 {
		log.Fatalf("Invalid value for [expiration_time]")
	}

	var policy webcache.Policy
	switch replacementPolicy {
	case LRU:
		policy = webcache.NewLRUPolicy()
	case LFU:
		policy = webcache.NewLFUPolicy()
	default:
		err = errors.New(fmt.Sprintf("Invalid cache replacement policy [%s]", replacementPolicy))
	}

	if err != nil {
		log.Fatal(err)
	}

	initializeDiskCache()
	initializeMMap()
	initializeWebCache(policy, cacheSize, expirationTime)

	client = &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				//LocalAddr: ipPort1,
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns: 100,
			IdleConnTimeout: 90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	log.Println("Starting HTTP proxy server")
	server := &http.Server{
		Addr:    ipPort1.String(),
		Handler: http.HandlerFunc(handleHTTP),
	}

	log.Println("Serving listening...")
	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}

}

func initializeDiskCache() {
	dc = webcache.NewDiskCache(CACHE_ROOT+"/diskcache", CACHE_ROOT+"/journal.log")
}

func initializeMMap() {

	invertedMap = &webcache.InvertedIndex{CACHE_ROOT+"/mmap", make(chan webcache.MappingRequest), make(chan webcache.Mapping)}
	loaded := make(chan struct{})
	go invertedMap.Run(loaded)
	<- loaded
}

func initializeWebCache(policy webcache.Policy, cacheSize uint64, expirationTime int) {
	wc = webcache.NewWebCache(policy, int(cacheSize), expirationTime)

	readChannel := make(chan *webcache.DiskCacheEntry)
	go dc.Read(readChannel)
	for entry := range readChannel {
		response := &webcache.Response {
			Body:           entry.Value,
			ContentType:    entry.ContentType,
			ExpirationTime: entry.ExpirationTime,
		}
		wc.Initialize(entry.Key, response)
	}
	wc.PrintCapacity()
}

func handleHTTP(w http.ResponseWriter, r *http.Request) {
	//TODO: remove this check later
	//if r.URL.String() == "http://detectportal.firefox.com/success.txt" {
	//	handleDefault(w, r)
	//	return
	//}
	switch r.Method {
	case GET:
		handleGet(w, r)
	default:
		//just pass on
		handleDefault(w, r)
	}
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	log.Println(fmt.Sprintf("GET Request - %s", r.URL.String()))
	url := removeCustomPrefix(r.URL.String())

	if _, ok := invertedMap.Get(webcache.Hash(url)); ok { url = webcache.Hash(url) } //get hashshed url if it exists on disk
	//if strings.HasPrefix(url, CUSTOM_URL_PREFIX) {
	//	url = HTTP_PREFIX + strings.TrimPrefix(url, CUSTOM_URL_PREFIX)
	//}

	response, err := wc.Get(url)
	var body []byte
	var contentType string
	if err != nil {
		log.Println(err.Error())
		log.Println(fmt.Sprintf("Requesting %s from server", url))

		mappedURL, ok := invertedMap.Get(url)
		if ok {
			log.Printf("Proxied GET for %s resolved to %s", url, mappedURL)
			url = mappedURL
		}

		resp, err := client.Get(url)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		if strings.HasPrefix(resp.Header.Get(CONTENT_TYPE), HTML_TYPE) {

			body, err = ReplaceURLs(resp.Body)
			if err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusServiceUnavailable) //TODO should probably be different here too
				return
			}
		} else {
			body, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusServiceUnavailable) //TODO should probably be different here too
				return
			}
		}
		contentType = resp.Header.Get(CONTENT_TYPE)
		invertedMap.NewMapping <- webcache.Mapping{Original: url, Hashed: webcache.Hash(strings.TrimPrefix(url, HTTP_PREFIX))}
		enterInCache(url, body, contentType, make(chan bool))
		resp.Body.Close()
	} else {
		log.Printf("HIT - %s", r.URL.String())
		body = response.Body
		contentType = response.ContentType
	}
	w.Header().Set(CONTENT_TYPE, contentType)
	w.Write(body)
}

func ReplaceURLs(body io.ReadCloser) ([]byte, error) {
	doneChannel := make(chan bool)
	resourceChannel := make(chan string)

	//Goroutine that will begin to fetch resources as we write them to the resource channel
	//When all resources are fetched, the done channel will be closed
	go getAllResources(resourceChannel, doneChannel)

	doc, err := html.Parse(body)
	if err != nil {
		return nil, errors.New("problem parsing html")
	}
	var f func(*html.Node) error
	f = func(n *html.Node) error {
		if n.Type == html.ElementNode && (n.Data == "img" || n.Data == "script") {
			for i, a := range n.Attr {
				if a.Key == "src" {
					src := a.Val

					//Only rewrite if it is an absolute link
					if strings.HasPrefix(src, HTTP_PREFIX) {
						//Send the resource to the channel so it will be fetched
						resourceChannel <- src
						//log.Println("Parsed link: ", n.Data, src)
						n.Attr[i].Val = createURL(src) //ipPort2.String() + strings.TrimPrefix(src, HTTP_PREFIX)
					}
					break
				}
			}
		} else if n.Type == html.ElementNode && n.Data == "link" {
			for i, a := range n.Attr {
				if a.Key == "href" {
					href := a.Val

					//Only rewrite if it is an absolute link
					if strings.HasPrefix(href, HTTP_PREFIX) {
						//Send the resource to the channel so it will be fetched
						resourceChannel <- href
						//log.Println("Parsed link: ", href)
						n.Attr[i].Val = createURL(href) //ipPort2.String() + strings.TrimPrefix(href, HTTP_PREFIX)
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			err = f(c)
			if err != nil {
				return err
			}
		}
		return nil
	}
	err = f(doc)
	if err != nil {
		return nil, err
	}

	//Close the resource channel after parsing is finished
	close(resourceChannel)

	//Wait for all the resources to be fetched before returning
	<- doneChannel
	var buf bytes.Buffer
	w := io.Writer(&buf)
	html.Render(w, doc)
	return buf.Bytes(), nil
}

func getAllResources(resources chan string, done chan bool) {
	defer close(done)

	//Start fetching each resource
	var resourcesFetched []chan bool
	for resource := range resources {
		doneChannel := make(chan bool)
		resourcesFetched = append(resourcesFetched, doneChannel)
		go getResource(resource, doneChannel)
	}

	//Wait for all resources to be fetched
	for _, resourceFetched := range resourcesFetched {
		<- resourceFetched
	}
}

func getResource(url string, done chan bool) error {
	trimmed := removeCustomPrefix(url)
	_, err := wc.Get(trimmed)
	if err != nil {
		//log.Println(err.Error())
		log.Println(fmt.Sprintf("Requesting resource %s from server", url))

		resp, err := client.Get(url)
		if err != nil {
			log.Println(err.Error())
			return err
		}
		defer resp.Body.Close()
		//log.Println(fmt.Sprintf("Successfully requested resource %s", url))
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		contentType := resp.Header.Get(CONTENT_TYPE)

		//Start a goroutine that will save the response to disk/cache
		go enterInCache(trimmed, body, contentType, done)
		return nil
	}
	return nil
}

func enterInCache(url string, body webcache.Value, contentType string, done chan bool) {
	defer close(done)

	//Find out what needs to be deleted
	toDelete, shouldCache := wc.FindEvictionEntries(url, body)

	//Delete from disk
	waitChannels := make(map[string]chan error)
	for _, elem := range toDelete {
		waitChannel := make(chan error)
		waitChannels[elem] = waitChannel
		dc.DeleteChannel <- &webcache.DiskCacheEntry{Key: elem, DoneChannel: waitChannel}
	}

	//Delete from web cache
	for key, waitChannel := range waitChannels {
		<-waitChannel
		wc.Delete(key)
	}

	if shouldCache {
		//Save entry to disk
		saveChannel := make(chan error)
		expiration := time.Now().Add(wc.ExpirationTime())
		dc.SaveChannel <- &webcache.DiskCacheEntry{
			Key:            webcache.Hash(url),
			Value:          body,
			ContentType:    contentType,
			ExpirationTime: expiration,
			DoneChannel:    saveChannel,
		}
		err := <-saveChannel

		//Save entry to web cache
		if err == nil {
			//Only save to web cache if save to disk was successful
			response := &webcache.Response{
				Body:           body,
				ContentType:    contentType,
				ExpirationTime: expiration,
			}
			wc.Set(url, response)
		} else {
			log.Println(fmt.Sprintf("Error saving %s to disk", url))
		}
	}
}

func handleDefault(w http.ResponseWriter, r *http.Request) {
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func getAddress(ip string) (*net.TCPAddr, error) {
	addr, err := net.ResolveTCPAddr("tcp", ip)
	return addr,  err
}

func createURL(src string) string {
	trimmed := strings.TrimPrefix(src, HTTP_PREFIX)
	hashed := webcache.Hash(trimmed)
	invertedMap.NewMapping <- webcache.Mapping{Original: src, Hashed: hashed}
	return fmt.Sprintf("%s://%s/%s", "http", ipPort2.String(), hashed)
}

func removeCustomPrefix(src string) string {
	customPrefix := fmt.Sprintf("%s://%s/", "http", ipPort2.String())
	if strings.HasPrefix(src, customPrefix) {
		return strings.TrimPrefix(src, customPrefix)
	} else if strings.HasPrefix(src, "/") { //Trick relative indicator / from hashed values
		return strings.TrimPrefix(src, "/")
	} else {
		return src
	}

}
