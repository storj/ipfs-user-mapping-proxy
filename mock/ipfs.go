package mock

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"

	"storj.io/ipfs-user-mapping-proxy/proxy"
)

// IPFSAddHandler is an HTTP handler that mocks the /api/v0/add enpoint of an IPFS Node.
func IPFSAddHandler(w http.ResponseWriter, r *http.Request) {
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	content, err := ioutil.ReadAll(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	hasher := sha256.New()
	_, err = hasher.Write([]byte(fileHeader.Filename))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	body, err := json.Marshal(&proxy.AddResponse{
		Name: fileHeader.Filename,
		Hash: base64.URLEncoding.EncodeToString(hasher.Sum(nil)),
		Size: strconv.Itoa(len(content)),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
