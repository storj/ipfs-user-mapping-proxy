package mock

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"storj.io/ipfs-user-mapping-proxy/proxy"
)

// IPFSDAGImportNoRootHandler is an HTTP handler that does not return root CID for the /api/v0/dag/import enpoint.
type IPFSDAGImportNoRootHandler struct{}

func (h *IPFSDAGImportNoRootHandler) Reset() {}

func (h *IPFSDAGImportNoRootHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	file, fileHeader, err := r.FormFile("file")
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
			Cid: map[string]string{
				"abc": Hash(fileHeader.Filename),
			},
		},
	})
	if err != nil {
		return
	}

	if proxy.Stats(r) {
		err = jw.Encode(proxy.DAGImportResponseMessage{
			Stats: &proxy.CarImportStats{
				BlockBytesCount: fileHeader.Size - 10,
				BlockCount:      1,
			},
		})
		if err != nil {
			return
		}
	}
}
