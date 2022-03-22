package main

import (
	"fmt"
	"log"

	"github.com/miekg/dns"
	"github.com/mitchellh/hashstructure/v2"
)

func main() {

	resolvers := []string{"8.8.8.8:53", "8.8.4.4:53", "1.1.1.1:53"}
	rcounter := 0

	cache := make(map[uint64]dns.Msg)

	dns.HandleFunc(".", func(w dns.ResponseWriter, req *dns.Msg) {

		if len(req.Question) < 1 {
			dns.HandleFailed(w, req)
			return
		}

		fmt.Printf("Got a request for %s\n", req.Question[0])

		hash, _ := hashstructure.Hash(req.Question, hashstructure.FormatV2, nil)

		resp, exists := cache[hash]
		if !exists {
			rcounter++
			if rcounter >= len(resolvers) {
				rcounter = 0
			}
			realresp, err := dns.Exchange(req, resolvers[rcounter])
			resp = *realresp

			if err != nil {
				fmt.Printf("Got an error %s\n", err)
				dns.HandleFailed(w, req)
				return
			}

			fmt.Printf("Got a response %s\n", resp)
			cache[hash] = resp
		} else {
			fmt.Printf("Got a response from cache: %s\n", resp)
			resp.Id = req.Id
		}

		if err := w.WriteMsg(&resp); err != nil {
			dns.HandleFailed(w, req)
			return
		}
	})

	log.Fatal(dns.ListenAndServe("127.0.0.1:53", "udp", nil))
}
