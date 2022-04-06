package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/kaloyan-raev/ipfs-user-mapping-proxy/db"
)

// AddResponse is the JSON object returned to Add requests.
type AddResponse struct {
	Name string `json:"Name"`
	Hash string `json:"Hash"`
	Size string `json:"Size"`
}

// HandleAdd is an HTTP handler that intercepts
// the /api/v0/add requests to the IPFS node.
//
// It retrieves the authenticated user from the requests and maps it to the
// uploaded content. The mapping is stored in the database.
func (p *Proxy) HandleAdd(w http.ResponseWriter, r *http.Request) {
	user, _, ok := r.BasicAuth()
	if !ok {
		http.Error(w, "no basic auth", http.StatusUnauthorized)
		return
	}

	wrapper := NewResponseWriterWrapper(w)
	p.proxy.ServeHTTP(wrapper, r)

	if wrapper.StatusCode != http.StatusOK {
		// add logging and montoring
		return
	}

	var resp AddResponse
	err := json.Unmarshal(wrapper.Body, &resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	size, err := strconv.ParseInt(resp.Size, 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = p.db.Add(context.Background(), db.Content{
		User: user,
		Hash: resp.Hash,
		Name: resp.Name,
		Size: size,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
