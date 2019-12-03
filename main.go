package main

import (
	"flag"
	"net/http"
)

var (
	listenAddr = flag.String("listener", "localhost:17391", "Address on which to host the proxy.")
	debug      = flag.Bool("debug", false, "Verbose logging.")
)

func main() {
	flag.Parse()

	c := openCache(flag.Arg(0))

	s := http.Server{
		Addr: *listenAddr,
		Handler: &proxy{
			debug:          *debug,
			cache:          c,
			getsInProgress: map[string]struct{}{},
		},
	}
	s.ListenAndServe()
}
