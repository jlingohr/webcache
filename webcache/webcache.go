package main

import (
	"io/ioutil"
	"net/http"
	"time"
)


type entry struct {
	res result
	ready chan struct{}
}

type result struct {
	value []byte
	err error
}

type request struct {
	url string
	response chan<- result
}

type WebCache struct {
	requests chan request
}

func NewWebCache(policy *Policy, cacheSize uint64, expirationTime uint8) *WebCache {
	requests := make(chan request)
	cache := &WebCache{requests: requests}
	go cache.server(policy, cacheSize, expirationTime)
	return cache
}

func (webcache *WebCache) Get(url string) ([]byte, error) {
	response := make(chan result)
	webcache.requests <- request{url, response}
	res := <- response
	return res.value, res.err
}

// Serve incoming GET requests
func (webcache *WebCache) server(policy *Policy, cacheSize uint64, expirationTime uint8) {
	cache := make(map[string]*entry)
	for req := range webcache.requests {
		e := cache[req.url]
		if e == nil {
			e = &entry{ready: make(chan struct{})}
			cache[req.url] = e
			go e.call(httpMakeRequest, req.url)
		}
		go e.deliver(req.response)
	}

}

// Evaluate the function f and broadcast the entry is ready
func (e *entry) call (f Func, key string) {
	e.res.value, e.res.err = f(key)
	close(e.ready)
}

// Wait for entry to be ready and send result to client
func (e *entry) deliver(response chan<- result) {
	<- e.ready
	response <- e.res
}

///////////////
// Functions

func httpMakeRequest(url string) ([]byte, error) {
	client := &http.Client{Timeout: time.Second*5,}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}