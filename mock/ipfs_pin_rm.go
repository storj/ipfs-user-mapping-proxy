package mock

import (
	"encoding/json"
	"net/http"

	"storj.io/ipfs-user-mapping-proxy/proxy"
)

// IPFSPinRmHandler is an HTTP handler that mocks the /api/v0/pin/rm enpoint of an IPFS Node.
type IPFSPinRmHandler struct {
	Invoked bool
	Removed []string
}

func (handler *IPFSPinRmHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler.Invoked = true

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

	handler.Removed = append(handler.Removed, toRemove...)

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
