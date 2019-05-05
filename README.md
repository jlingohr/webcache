# Webcache

A web cache that caches and serves static web content retrieved by a browser using HTTP GETs and serves multiple clients concurrently. Has persistent state to recover from crashes or restarts.

`go run web-cache.go [ip1:port] [ip2:port] [replacement_policy] [cache_size] [expiration_time]`

* [ip1:port1] : The TCP IP address and the port that the web cache will bind to to accept connections from clients. The web cache should also bind to ip1 when connecting to remote web servers to retrieve resources on behalf of clients.
* [ip2:port2] : The TCP IP address and the port that the web cache should use when rewriting the HTML.
* [replacement_policy] : The replacement policy ("LRU" or "LFU") that the web cache follows during eviction.
* [cache_size] : The capacity of the cache in MB (your cache cannot use more than this amount of capacity). Note that this specifies the (same) capacity for both the memory cache and the disk cache.
* [expiration_time] : The time period in seconds after which an item in the cache is considered to be expired.
