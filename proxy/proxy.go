package proxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/ipfs-user-mapping-proxy/db"
)

var mon = monkit.Package()

// Proxy is a reverse proxy to the IPFS node's HTTP API that
// maps uploaded content to the authenticated user.
type Proxy struct {
	address string
	proxy   *httputil.ReverseProxy
	db      *db.DB
}

// New creates a new Proxy to target. Proxy listens on the provided address
// and stores the mappings to db.
func New(address string, target *url.URL, db *db.DB) *Proxy {
	return &Proxy{
		address: address,
		proxy:   httputil.NewSingleHostReverseProxy(target),
		db:      db,
	}
}

// Run starts the proxy.
func (p *Proxy) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	http.HandleFunc("/api/v0/add", p.HandleAdd)

	return http.ListenAndServe(p.address, nil)
}
