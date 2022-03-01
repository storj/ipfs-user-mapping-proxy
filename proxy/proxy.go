package proxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/kaloyan-raev/ipfs-user-mapping-proxy/db"
)

// Proxy is a reverse proxy to the IPFS node's HTTP API that
// maps uploaded content to the authenticated user.
type Proxy struct {
	address string
	proxy   *httputil.ReverseProxy
	db      *pgxpool.Pool
}

// New creates a new Proxy to target. Proxy listens on the provided address
// and stores the mappings to db.
func New(address string, target *url.URL, db *pgxpool.Pool) *Proxy {
	return &Proxy{
		address: address,
		proxy:   httputil.NewSingleHostReverseProxy(target),
		db:      db,
	}
}

// Run starts the proxy.
func (p *Proxy) Run(ctx context.Context) error {
	err := db.Init(ctx, p.db)
	if err != nil {
		return err
	}

	http.HandleFunc("/api/v0/add", p.HandleAdd)

	return http.ListenAndServe(p.address, nil)
}
