package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/mitchellh/hashstructure/v2"
)

type cacheExpEntry struct {
	Value   *dns.Msg
	Expires time.Time
}

func NewCacheEntry(msg *dns.Msg) *cacheExpEntry {
	entry := new(cacheExpEntry)

	entry.Value = msg
	if msg.Rcode != dns.RcodeSuccess {
		fmt.Printf("Negative cache for %s", msg)
		// negative cache
		entry.Expires = time.Now().Add(time.Second * 30)
	} else {
		ttl := 86400
		for _, answer := range msg.Answer {
			if int(answer.Header().Ttl) < ttl {
				ttl = int(answer.Header().Ttl)
			}
		}
		entry.Expires = time.Now().Add(time.Second * time.Duration(ttl))
	}

	return entry
}
func (entry *cacheExpEntry) Expired() bool {
	expired := entry.Expires.Before(time.Now())
	return expired
}

type requestStats struct {
	Total     int64
	Forwarded int64
	Cached    int64
}

func (stats *requestStats) PrintStats() {
	if stats.Total%10 == 1 {
		fmt.Printf("Cache hit ratio is %.2f\n", float64(stats.Cached)/float64(stats.Total))
	}
}

func (stats *requestStats) GetResolver() string {
	resolvers := []string{"8.8.8.8:53", "8.8.4.4:53", "1.1.1.1:53"}
	index := stats.Total % int64(len(resolvers))
	return resolvers[index]
}

type Cache struct {
	mu    *sync.RWMutex
	cache map[uint64]*cacheExpEntry
}

func NewCache() Cache {
	return Cache{
		mu:    &sync.RWMutex{},
		cache: make(map[uint64]*cacheExpEntry, 1000),
	}
}

func (c Cache) Set(hash uint64, entry *cacheExpEntry) {
	c.mu.Lock()
	//fmt.Println("map update")
	defer c.mu.Unlock()
	c.cache[hash] = entry
}

func (c Cache) Get(hash uint64) (*cacheExpEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.cache[hash]
	/*
		We do not really need this - we are overwiting the expired entry later anyway.
		So not doing this here gives us the same benefits, but one fewer write lock.
		if ok && entry.Expired() {
			ok = false
			c.mu.Lock()
			defer c.mu.Unlock()
			delete(c.cache, hash)
		}
	*/
	return entry, ok
}

func main() {

	stats := requestStats{0, 0, 0}

	cache := NewCache()

	dns.HandleFunc(".", func(w dns.ResponseWriter, req *dns.Msg) {

		if len(req.Question) < 1 {
			dns.HandleFailed(w, req)
			return
		}

		//fmt.Printf("Got a request for %s\n", req.Question[0])
		stats.PrintStats()

		stats.Total++

		hash, _ := hashstructure.Hash(req.Question, hashstructure.FormatV2, nil)

		resp, exists := cache.Get(hash)

		if !exists || resp.Expired() {
			stats.Forwarded++
			realresp, err := dns.Exchange(req, stats.GetResolver())

			if err != nil {
				fmt.Printf("Got an error %s\n", err)
				dns.HandleFailed(w, req)
				// Do not cache error reponse - this is not a DNS error, it is a timeout.
				// cache.Set(hash, NewCacheEntry(nil))
				return
			}

			resp = NewCacheEntry(realresp)

			cache.Set(hash, resp)
		} else {
			stats.Cached += 1
			resp.Value.Id = req.Id
		}

		if err := w.WriteMsg(resp.Value); err != nil {
			dns.HandleFailed(w, req)
			return
		}
	})

	log.Fatal(dns.ListenAndServe("127.0.0.1:53", "udp", nil))
}
