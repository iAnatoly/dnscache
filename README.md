# Very Simple DNS cache server

Very siomple DNS cache server.

Resolves DNS queries against in-memory cache; cache misses are forwarded to a ghardcoded list of public recursive resolvers. 

Known issues: 
* TTL is ignored

TODO:
1. Implement LRU cache
2. Implement TTL
3. Implement DoH

