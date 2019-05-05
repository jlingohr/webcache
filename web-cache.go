package A2_x4w7_c7l0b

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"webcache/webcache"
)

var(
	wc *webcache.WebCache
)

const GET = "GET"

func main() {
	args := os.Args[1:]

	if len(args) != 4 {
		fmt.Print("Usage: web-cache.go [ip:port] [replacement_policy] [cache_size] [expiration_time]")
		return
	}

	ipPort := args[0]
	replacementPolicy := args[1]
	cacheSize, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		log.Fatal("Invalid parameter [cache_size]")
	}
	expirationTime, err := strconv.ParseUint(args[3], 10, 8)
	if err != nil {
		return
	}

	policy, err := webcache.NewPolicy(replacementPolicy, uint8(expirationTime))
	if err != nil {
		log.Fatal(err)
	}

	wc = webcache.NewWebCache(policy, cacheSize, uint8(expirationTime))

	log.Println("Starting HTTP proxy server")
	server := &http.Server{
		Addr: ipPort,
		Handler: http.HandlerFunc(handleHTTP),
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}


}

func handleHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case GET:
		body, err := wc.Get(r.URL.String())
		if err != nil {
			log.Println(err)
		}
		w.Write(body)
	default:
		//just pass on
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

}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k,v)
		}
	}
}
