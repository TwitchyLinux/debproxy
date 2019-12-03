package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

type proxy struct {
	c     http.Client
	debug bool

	cache          *cache
	lock           sync.Mutex
	getsInProgress map[string]struct{}
}

func (p *proxy) multiDownload(ctx context.Context, req *http.Request, outputs ...io.Writer) error {
	proxyReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL.String(), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewRequestWithContext() failed: %v\n", err)
		return err
	}

	resp, err := p.c.Do(proxyReq)
	if err != nil {
		fmt.Fprintf(os.Stderr, "proxy request failed: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	for _, w := range outputs {
		if w, ok := w.(http.ResponseWriter); ok {
			if resp.ContentLength >= 0 {
				w.Header().Set("Content-Length", fmt.Sprint(resp.ContentLength))
			}
			if ct := resp.Header.Get("Content-Type"); ct != "" {
				w.Header().Set("Content-Type", ct)
			}
			w.WriteHeader(resp.StatusCode)
		}
	}

	_, err = io.Copy(io.MultiWriter(outputs...), resp.Body)
	return err
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if p.debug {
		fmt.Fprintf(os.Stdout, "[%s] [%s] %q\n", req.Method, req.URL.Host, req.URL.Path)
	}
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, req.Method+" not supported", http.StatusBadRequest)
		return
	}

	// Just direct proxy everything that isn't a deb.
	if !shouldProxy(req.URL) {
		p.multiDownload(req.Context(), req, w)
		return
	}

	// Try and serve from cache.
	d := p.cache.Get(req.URL.String())
	if len(d) > 0 {
		w.Header().Set("Content-Length", fmt.Sprint(len(d)))
		// w.Header().Set("Content-Type", )
		io.Copy(w, bytes.NewReader(d))
		return
	}

	// If the cache is already being warmed for that key, just serve it.
	p.lock.Lock()
	_, keyBeingDownloaded := p.getsInProgress[req.URL.String()]
	p.lock.Unlock()
	if keyBeingDownloaded {
		p.multiDownload(req.Context(), req, w)
		return
	}

	// Otherwise download to both the client and the cache.
	p.lock.Lock()
	p.getsInProgress[req.URL.String()] = struct{}{}
	p.lock.Unlock()
	defer func() {
		p.lock.Lock()
		delete(p.getsInProgress, req.URL.String())
		p.lock.Unlock()
	}()

	var contents bytes.Buffer
	contents.Grow(1024 * 32)
	if err := p.multiDownload(req.Context(), req, w, &contents); err == nil {
		select {
		case <-req.Context().Done():
		default:
			p.cache.Put(req.URL.String(), contents.Bytes())
		}
	}
}

func shouldProxy(url *url.URL) bool {
	switch {
	case strings.HasSuffix(url.Path, ".deb"):
		return true
	case strings.Contains(url.Path, "/by-hash/SHA256/"):
		return true
	}
	return false
}
