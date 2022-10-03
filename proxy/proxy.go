package proxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/ipfs-user-mapping-proxy/db"
)

var mon = monkit.Package()

const (
	AddEndpoint   = "/api/v0/add"
	PinLsEndpoint = "/api/v0/pin/ls"
	PinRmEndpoint = "/api/v0/pin/rm"
)

// Proxy is a reverse proxy to the IPFS node's HTTP API that
// maps uploaded content to the authenticated user.
type Proxy struct {
	log     *zap.Logger
	db      *db.DB
	address string
	target  *url.URL
	proxy   *httputil.ReverseProxy
}

// New creates a new Proxy to target. Proxy listens on the provided address
// and stores the mappings to db.
func New(log *zap.Logger, db *db.DB, address string, target *url.URL) *Proxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		log.Error("Proxy error", zap.Error(err))
		rw.WriteHeader(http.StatusBadGateway)
	}

	return &Proxy{
		log:     log,
		db:      db,
		address: address,
		target:  target,
		proxy:   proxy,
	}
}

// Run starts the proxy.
func (p *Proxy) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return http.ListenAndServe(p.address, p.ServeMux())
}

func (p *Proxy) ServeMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(AddEndpoint, p.HandleAdd)
	mux.HandleFunc(PinLsEndpoint, p.HandlePinLs)
	mux.HandleFunc(PinRmEndpoint, p.HandlePinRm)
	return mux
}
