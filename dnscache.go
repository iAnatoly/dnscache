package main

import (
	"fmt"
	"log"
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
	if msg == nil {
		// negative cache
		entry.Expires = time.Now().Add(time.Second * 30)
		entry.Value = nil
	} else {
		ttl := 86400
		for _, answer := range msg.Answer {
			if int(answer.Header().Ttl) < ttl {
				ttl = int(answer.Header().Ttl)
			}
		}
		entry.Expires = time.Now().Add(time.Second * time.Duration(ttl))
		entry.Value = msg
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

func main() {

	stats := requestStats{0, 0, 0}

	cache := make(map[uint64]*cacheExpEntry)

	dns.HandleFunc(".", func(w dns.ResponseWriter, req *dns.Msg) {

		if len(req.Question) < 1 {
			dns.HandleFailed(w, req)
			return
		}

		// fmt.Printf("Got a request for %s\n", req.Question[0])
		stats.PrintStats()

		stats.Total++

		hash, _ := hashstructure.Hash(req.Question, hashstructure.FormatV2, nil)

		resp, exists := cache[hash]

		if !exists || resp.Expired() {
			stats.Forwarded++
			realresp, err := dns.Exchange(req, stats.GetResolver())

			if err != nil {
				fmt.Printf("Got an error %s\n", err)
				dns.HandleFailed(w, req)
				cache[hash] = NewCacheEntry(nil)
				return
			}

			resp = NewCacheEntry(realresp)

			cache[hash] = resp
		} else {
			stats.Cached += 1
			if resp.Value == nil {
				dns.HandleFailed(w, req)
				fmt.Printf("Negative cache for %s\n", req.Question[0])
				return
			}

			resp.Value.Id = req.Id
		}

		if err := w.WriteMsg(resp.Value); err != nil {
			dns.HandleFailed(w, req)
			return
		}
	})

	log.Fatal(dns.ListenAndServe("127.0.0.1:53", "udp", nil))
}
