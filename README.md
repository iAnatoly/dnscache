# Very Simple DNS cache server

Very siomple DNS cache server.

Resolves DNS queries against in-memory LRU cache; cache misses are forwarded to a ghardcoded list of public recursive resolvers. 

TODO:
1. [DONE] Implement LRU cache
2. [DONE] Implement TTL
3. [DONE] Implement negative cache (after TTL)
4. Implement DoH

