package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/ipfs-user-mapping-proxy/db"
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
	code, err := p.handleAdd(r.Context(), w, r)

	mon.Counter("add_handler_response_codes", monkit.NewSeriesTag("code", strconv.Itoa(code))).Inc(1)

	if err != nil {
		http.Error(w, err.Error(), code)
		return
	}
}

func (p *Proxy) handleAdd(ctx context.Context, w http.ResponseWriter, r *http.Request) (code int, err error) {
	defer mon.Task()(&ctx)(&err)

	user, _, ok := r.BasicAuth()
	if !ok {
		return http.StatusUnauthorized, errors.New("no basic auth")
	}

	wrapper := NewResponseWriterWrapper(w)
	p.proxy.ServeHTTP(wrapper, r)

	if wrapper.StatusCode != http.StatusOK {
		return wrapper.StatusCode, nil
	}

	var resp AddResponse
	err = json.Unmarshal(wrapper.Body, &resp)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	size, err := strconv.ParseInt(resp.Size, 10, 64)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	err = p.db.Add(ctx, db.Content{
		User: user,
		Hash: resp.Hash,
		Name: resp.Name,
		Size: size,
	})
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return wrapper.StatusCode, nil
}
