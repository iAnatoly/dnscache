package main

import (
	"fmt"
	"log"
	"time"

	lru "github.com/hashicorp/golang-lru"
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
		// negative cache
		fmt.Printf("Negative cache for %s", msg.Question)
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
		fmt.Printf("Cache hit ratio is %.2f for %d total queries\n", float64(stats.Cached)/float64(stats.Total), stats.Total)
	}
}

func (stats *requestStats) GetResolver() string {
	resolvers := []string{"8.8.8.8:53", "8.8.4.4:53", "1.1.1.1:53"}
	index := stats.Total % int64(len(resolvers))
	return resolvers[index]
}

type Cache struct {
	cache *lru.Cache
}

func NewCache(size int) Cache {
	lruCache, _ := lru.New(size)
	return Cache{
		cache: lruCache,
	}
}

func (c Cache) Set(hash uint64, entry *cacheExpEntry) {
	if c.cache.Contains(hash) {
		c.cache.Remove(hash)
	}
	c.cache.Add(hash, entry)
}

func (c Cache) Get(hash uint64) (*cacheExpEntry, bool) {
	entry, ok := c.cache.Get(hash)
	if ok {
		return entry.(*cacheExpEntry), true
	}
	return nil, false
}

func main() {

	stats := requestStats{0, 0, 0}
	cache := NewCache(10000)

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
				// Do not cache error response - this is not a DNS error, it is a timeout.
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
