package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"
)

// PinRmResponseMessage is the JSON object returned to Pin Remove requests.
type PinRmResponseMessage struct {
	Pins []string `json:"Pins"`
}

// HandlePinRm is an HTTP handler that intercepts
// the /api/v0/pin/rm requests to the IPFS node.
//
// It retrieves the authenticated user from the requests and maps it to the
// unpinned content. The mapping is stored in the database.
func (p *Proxy) HandlePinRm(w http.ResponseWriter, r *http.Request) {
	_ = p.handlePinRm(r.Context(), w, r)
}

func (p *Proxy) handlePinRm(ctx context.Context, w http.ResponseWriter, r *http.Request) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, _, ok := r.BasicAuth()
	if !ok {
		mon.Counter("pin_rm_handler_response_codes", monkit.NewSeriesTag("code", strconv.Itoa(http.StatusUnauthorized))).Inc(1)
		p.log.Error("No basic auth in request")
		err = errors.New("no basic auth")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	var toRemove []string
	for param, value := range r.URL.Query() {
		switch param {
		case "arg":
			toRemove = append(toRemove, value...)
			continue
		default:
			mon.Counter("pin_rm_handler_invalid_query_param", monkit.NewSeriesTag("param", param)).Inc(1)
			p.log.Error("Invalid query param",
				zap.String("User", user),
				zap.String("Param", param))
			err = errors.New("only arg arguments are allowed")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return err
		}
	}

	if len(toRemove) == 0 {
		mon.Counter("pin_rm_handler_no_args").Inc(1)
		p.log.Error("No args", zap.String("User", user))
		err = errors.New(`argument "ipfs-path" is required`)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	checkArgs := sliceToSet(toRemove)
	backendArgs := sliceToSet(toRemove)

	// Check if user pinned this content and remove it from the DB.
	userHashes, err := p.db.ListActiveContentByHash(ctx, toRemove)
	if err != nil {
		mon.Counter("pin_rm_handler_error_db_list_content").Inc(1)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	for _, userHash := range userHashes {
		if userHash.User != user {
			// Another user pinned the same hash. Remove it from backendArgs.
			delete(backendArgs, userHash.Hash)
			continue
		}
		// The authenticated user has this hash pinned. Remove it from the checkArgs.
		delete(checkArgs, userHash.Hash)
	}

	// If checkArgs is still not empty, the user requested to remove content that they haven't pinned.
	if len(checkArgs) > 0 {
		mon.Counter("pin_rm_handler_error_content_not_pinned").Inc(1)
		err := fmt.Errorf("not pinned or pinned indirectly: %s", setToSlice(checkArgs))
		http.Error(w, err.Error(), http.StatusNotFound)
		return err
	}

	// Remove the requested pins from the database.
	err = p.db.RemoveContentByHashForUser(ctx, user, toRemove)
	if err != nil {
		mon.Counter("pin_rm_handler_error_db_remove_content").Inc(1)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	if len(backendArgs) == 0 {
		// All content requested for removal is pinned by other users.
		// No need to request the backend. Just send a success response back to the client.
		return writeResponse(w, toRemove)
	}

	// Request the backend with only the hashes remaining in backendArg.
	u := r.URL
	u.Scheme = p.target.Scheme
	u.Host = p.target.Host
	u.RawQuery = url.Values(map[string][]string{
		"arg": setToSlice(backendArgs),
	}).Encode()

	resp, err := http.DefaultClient.Post(u.String(), "", nil)
	if err != nil {
		// Log the error but don't return error to the client.
		mon.Counter("pin_rm_handler_error_backend_request").Inc(1)
		p.log.Error("Error requesting backend", zap.Error(err))
		// Send a success response.
		return writeResponse(w, toRemove)
	}

	code := resp.StatusCode
	mon.Counter("pin_rm_handler_response_codes", monkit.NewSeriesTag("code", strconv.Itoa(code))).Inc(1)

	if code != http.StatusOK {
		// The backend responded with error - relay it back to the client.
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Set(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, err := io.Copy(w, resp.Body)
		return err
	}

	// Discard the response body from the backend.
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		// Log the error but don't return error to the client.
		mon.Counter("pin_rm_handler_error_discard_backend_respond").Inc(1)
		p.log.Error("Error discarding backend response", zap.Error(err))
		// Send a success response.
		return writeResponse(w, toRemove)
	}

	// Send our own success response.
	return writeResponse(w, toRemove)
}

func writeResponse(w http.ResponseWriter, pins []string) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(PinRmResponseMessage{Pins: pins})
}

func sliceToSet(slice []string) (set map[string]struct{}) {
	set = make(map[string]struct{})
	for _, item := range slice {
		set[item] = struct{}{}
	}
	return set
}

func setToSlice(set map[string]struct{}) (slice []string) {
	for item := range set {
		slice = append(slice, item)
	}
	return slice
}
