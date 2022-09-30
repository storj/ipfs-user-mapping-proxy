package mock

import (
	"encoding/json"
	"net/http"
	"sort"

	"storj.io/ipfs-user-mapping-proxy/proxy"
)

// IPFSPinRmHandler is an HTTP handler that mocks the /api/v0/pin/rm enpoint of an IPFS Node.
type IPFSPinRmHandler struct {
	Invoked bool
	Removed []string
}

func (h *IPFSPinRmHandler) Reset() {
	h.Invoked = false
	h.Removed = nil
}

func (h *IPFSPinRmHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Invoked = true

	var toRemove []string
	for param, value := range r.URL.Query() {
		switch param {
		case "arg":
			toRemove = append(toRemove, value...)
			continue
		default:
			continue
		}
	}

	if len(toRemove) == 0 {
		http.Error(w, `argument "ipfs-path" is required`, http.StatusBadRequest)
		return
	}

	sort.Strings(toRemove)
	h.Removed = append(h.Removed, toRemove...)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jw := json.NewEncoder(w)

	err := jw.Encode(proxy.PinRmResponseMessage{
		Pins: toRemove,
	})
	if err != nil {
		panic(err)
	}
}
