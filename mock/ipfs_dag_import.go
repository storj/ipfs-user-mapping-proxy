package mock

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"storj.io/ipfs-user-mapping-proxy/proxy"
)

// IPFSDAGImportHandler is an HTTP handler that mocks the /api/v0/dag/import enpoint of an IPFS Node.
type IPFSDAGImportHandler struct{}

func (h *IPFSDAGImportHandler) Reset() {}

func (h *IPFSDAGImportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jw := json.NewEncoder(w)

	var (
		blockCount int64
		bytesCount int64
	)

	// Trigger parsing multipart form
	_ = r.FormValue("")

	for _, files := range r.MultipartForm.File {
		for _, fh := range files {
			f, err := fh.Open()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			_, err = io.Copy(ioutil.Discard, f)
			if err != nil {
				return
			}

			err = jw.Encode(proxy.DAGImportResponseMessage{
				Root: &proxy.RootMeta{
					Cid: map[string]string{
						"/": Hash(fh.Filename),
					},
				},
			})
			if err != nil {
				return
			}

			blockCount++
			bytesCount += fh.Size - 10
		}
	}

	if proxy.Stats(r) {
		err := jw.Encode(proxy.DAGImportResponseMessage{
			Stats: &proxy.CarImportStats{
				BlockBytesCount: bytesCount,
				BlockCount:      blockCount,
			},
		})
		if err != nil {
			return
		}
	}
}
