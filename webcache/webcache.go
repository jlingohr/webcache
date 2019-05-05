package webcache

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

type Value []byte

type Cache interface {
	Get(url string)  (*Response, error)
	Delete(key string)
	Set(url string, response *Response)
	FindEvictionEntries(url string, value Value)([]string, bool)
	Initialize(key string, value *Response)
	ExpirationTime() time.Duration
	PrintCapacity()
}

type WebCache struct {
	pendingSet int
	currentCapacity int
	maxCapacity int
	expirationTime time.Duration
	policy      Policy
	sync.RWMutex
	cache      map[string]*Entry
	updateChan chan *Entry
}


func NewWebCache(policy Policy, cacheSize int, expirationTime int) Cache {

	c := &WebCache{
		currentCapacity: 0,
		pendingSet: 0,
		maxCapacity: cacheSize*1000000,
		expirationTime: time.Duration(expirationTime)*time.Second,
		cache:          make(map[string]*Entry),
		updateChan:     make(chan *Entry),
		policy:         policy,
	}

	return c
}

func (c *WebCache) ExpirationTime() time.Duration { return c.expirationTime }

func (c *WebCache) Get(url string) (*Response, error) {
	c.RLock()
	defer c.RUnlock()

	entry, ok := c.cache[RemoveHTTPPrefix(url)]
	if !ok {
		return nil, errors.New(fmt.Sprintf("MISS - %s", url))
	}
	if !entry.Expired() {
		c.promote(entry)
		//log.Println(fmt.Sprintf("HIT - %s", url))
		return entry.Response, nil //TODO this response is not being read properly
	} else {
		return nil, errors.New(fmt.Sprintf("EXPIRED - %s", url))
	}
}

func (c *WebCache) FindEvictionEntries(url string, value Value) (toDelete []string, cache bool) {
	c.Lock()
	defer c.Unlock()

	length := len(value)

	if length > c.maxCapacity {
		log.Println(fmt.Sprintf("Not Caching - Response for %s too large.", url))
		return toDelete, false
	} else if (c.maxCapacity - (c.currentCapacity + c.pendingSet)) < length {
		//log.Println(fmt.Sprintf("Need to make room for %s in the cache. Start evicting.", url))
		room := c.maxCapacity - (c.currentCapacity + c.pendingSet)
		for room < length {
			toEvict := c.policy.Evict()
			if toEvict == nil {
				log.Println(fmt.Sprintf("Not Caching - Unable to make room for %s.", url))
				return toDelete, false
			}
			toDelete = append(toDelete, toEvict.Key)
			room += toEvict.Size
		}
	}
	c.pendingSet += length
	return toDelete, true
}

func (c *WebCache) Delete(key string) {
	c.Lock()
	defer c.Unlock()

	if c.cache[key] != nil {
		size := c.cache[key].Size
		delete(c.cache, key)
		c.currentCapacity -= size
		log.Println(fmt.Sprintf("EVICT - %s", key))
		c.PrintCapacity()
	}
}

func (c *WebCache) Set(url string, value *Response) {
	c.Lock()
	defer c.Unlock()
	hash := Hash(url)
	entry := NewEntry(hash, value)

	//Only add to the cache size if the entry isn't in the cache already
	if c.cache[hash] == nil {
		c.currentCapacity += entry.Size
		c.pendingSet -= entry.Size
		log.Println(fmt.Sprintf("SET - URL: %s Key: %s", url, hash))
	} else {
		log.Println(fmt.Sprintf("UPDATE - URL: %s Key: %s", url, hash))
		c.pendingSet -= entry.Size
	}
	c.cache[hash] = entry
	c.promote(entry)
	c.PrintCapacity()
}

func (c *WebCache) Initialize(key string, value *Response) {
	log.Println(fmt.Sprintf("Adding disk cache entry to web cache. Key: %s", key))
	entry := NewEntry(key, value)
	c.cache[key] = entry
	c.currentCapacity += entry.Size
	c.promote(entry)
}

func (c *WebCache) promote(entry *Entry) {
	c.policy.Promote(entry)
}

func (c *WebCache) PrintCapacity() {
	CacheStatus(c.currentCapacity, c.maxCapacity)
}

func Hash(key string) string {
	// First strip prefix
	url := RemoveHTTPPrefix(key)
	hash := sha256.New()
	hash.Write([]byte(url))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func RemoveHTTPPrefix(src string) string {
	return strings.Trim(src, "http://")
}
