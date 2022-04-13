package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

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
	_ = p.handleAdd(r.Context(), w, r)
}

func (p *Proxy) handleAdd(ctx context.Context, w http.ResponseWriter, r *http.Request) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, _, ok := r.BasicAuth()
	if !ok {
		mon.Counter("add_handler_response_codes", monkit.NewSeriesTag("code", strconv.Itoa(http.StatusUnauthorized))).Inc(1)
		p.log.Error("No basic auth in request")
		err = errors.New("no basic auth")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	wrapper := NewResponseWriterWrapper(w)
	p.proxy.ServeHTTP(wrapper, r)

	code := wrapper.StatusCode
	mon.Counter("add_handler_response_codes", monkit.NewSeriesTag("code", strconv.Itoa(code))).Inc(1)

	if code != http.StatusOK {
		if code > 400 && code != http.StatusBadGateway {
			// BadGateway is logged by the proxy error handler
			p.log.Error("Proxy error", zap.Int("Code", code), zap.ByteString("Body", wrapper.Body))
		}
		return err
	}

	var resp AddResponse
	err = json.Unmarshal(wrapper.Body, &resp)
	if err != nil {
		mon.Counter("error_unmarshal_response").Inc(1)
		p.log.Error("JSON response unmarshal error", zap.ByteString("Body", wrapper.Body), zap.Error(err))
		return err
	}

	size, err := strconv.ParseInt(resp.Size, 10, 64)
	if err != nil {
		mon.Counter("error_parse_size").Inc(1)
		p.log.Error("Size parse error", zap.String("Size", resp.Size), zap.Error(err))
		return err
	}

	err = p.db.Add(ctx, db.Content{
		User: user,
		Hash: resp.Hash,
		Name: resp.Name,
		Size: size,
	})
	if err != nil {
		mon.Counter("error_db_add").Inc(1)
		p.log.Error("Error adding content to database",
			zap.String("User", user),
			zap.String("Hash", resp.Hash),
			zap.String("Name", resp.Name),
			zap.Int64("Size", size),
			zap.Error(err))
		return err
	}

	return nil
}
