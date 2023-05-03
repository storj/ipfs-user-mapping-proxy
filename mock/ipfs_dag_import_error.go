package mock

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"storj.io/ipfs-user-mapping-proxy/proxy"
)

// IPFSDAGImportErrorHandler is an HTTP handler that returns PinErrorMsg for the /api/v0/dag/import enpoint.
type IPFSDAGImportErrorHandler struct{}

func (h *IPFSDAGImportErrorHandler) Reset() {}

func (h *IPFSDAGImportErrorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jw := json.NewEncoder(w)

	_, err = io.Copy(ioutil.Discard, file)
	if err != nil {
		return
	}

	err = jw.Encode(proxy.DAGImportResponseMessage{
		Root: &proxy.RootMeta{
			PinErrorMsg: "error",
		},
	})
	if err != nil {
		return
	}
}
