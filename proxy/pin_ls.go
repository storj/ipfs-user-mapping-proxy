package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"
)

// PinLsResponseMessage is the JSON object returned to Pin List requests.
type PinLsResponseMessage struct {
	Keys map[string]interface{} `json:"Keys"`
}

// HandlePinLs is an HTTP handler that intercepts
// the /api/v0/pin/ls requests to the IPFS node.
//
// It retrieves the authenticated user from the requests and maps it to the
// pinned content. The mapping is stored in the database.
func (p *Proxy) HandlePinLs(w http.ResponseWriter, r *http.Request) {
	_ = p.handlePinLs(r.Context(), w, r)
}

func (p *Proxy) handlePinLs(ctx context.Context, w http.ResponseWriter, r *http.Request) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, _, ok := r.BasicAuth()
	if !ok {
		mon.Counter("pin_ls_handler_response_codes", monkit.NewSeriesTag("code", strconv.Itoa(http.StatusUnauthorized))).Inc(1)
		p.log.Error("No basic auth in request")
		err = errors.New("no basic auth")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	for param := range r.URL.Query() {
		switch param {
		default:
			mon.Counter("pin_ls_handler_invalid_query_param", monkit.NewSeriesTag("param", param)).Inc(1)
			p.log.Error("Invalid query param",
				zap.String("User", user),
				zap.String("Param", param))
			err = errors.New("no arguments are allowed")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return err
		}
	}

	// List the pinned content for this user from the DB.
	hashes, err := p.db.ListActiveContentByUser(ctx, user)
	if err != nil {
		mon.Counter("pin_ls_handler_error_db_list_content").Inc(1)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	keys := make(map[string]interface{}, len(hashes))
	for _, hash := range hashes {
		keys[hash] = map[string]string{
			"Type": "recursive",
		}
	}

	// Write the response.
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(PinLsResponseMessage{Keys: keys})
}
