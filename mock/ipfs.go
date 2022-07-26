package mock

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strconv"

	"storj.io/ipfs-user-mapping-proxy/proxy"
)

// IPFSAddHandler is an HTTP handler that mocks the /api/v0/add enpoint of an IPFS Node.
func IPFSAddHandler(w http.ResponseWriter, r *http.Request) {
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.URL.Query().Has("cid-version") {
		switch v := r.URL.Query().Get("cid-version"); v {
		case "":
			http.Error(w, "empty value not allowed for cid-version", http.StatusBadRequest)
			return
		case "0":
			break
		case "1":
			break
		default:
			http.Error(w, fmt.Sprintf("value '%s' not allowed for cid-version", v), http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	var totalSize int
	var hasher hash.Hash
	if isDir(fileHeader) {
		totalSize, hasher, err = processDir(r, w)
	} else {
		totalSize, hasher, err = processFile(w, file, fileHeader)
	}
	if err != nil {
		panic(err)
	}

	if !proxy.WrapWithDirectory(r) {
		return
	}

	hasher.Write([]byte(" (wrapped)"))
	totalSize += len(fileHeader.Filename)

	wrapped, err := json.Marshal(&proxy.AddResponseMessage{
		Hash: base64.URLEncoding.EncodeToString(hasher.Sum(nil)),
		Size: strconv.Itoa(totalSize),
	})
	if err != nil {
		panic(err)
	}

	_, err = w.Write(wrapped)
	if err != nil {
		panic(err)
	}
}

func isDir(header *multipart.FileHeader) bool {
	return header.Header.Get("Content-Type") == "application/x-directory"
}

func processDir(r *http.Request, w http.ResponseWriter) (int, hash.Hash, error) {
	var folderName string
	var totalSize int
	for i, fh := range r.MultipartForm.File["file"] {
		if i == 0 {
			folderName = fh.Filename
			continue
		}

		f, err := fh.Open()
		if err != nil {
			return 0, nil, err
		}

		size, _, err := processFile(w, f, fh)
		if err != nil {
			return 0, nil, err
		}

		totalSize += size
	}

	totalSize += len(folderName)

	hasher := sha256.New()
	_, err := hasher.Write([]byte(folderName))
	if err != nil {
		return 0, nil, err
	}

	body, err := json.Marshal(&proxy.AddResponseMessage{
		Name: folderName,
		Hash: base64.URLEncoding.EncodeToString(hasher.Sum(nil)),
		Size: strconv.Itoa(totalSize),
	})
	if err != nil {
		return 0, nil, err
	}

	_, err = w.Write(body)
	if err != nil {
		return 0, nil, err
	}

	return totalSize, hasher, nil
}

func processFile(w http.ResponseWriter, file multipart.File, header *multipart.FileHeader) (int, hash.Hash, error) {
	_, err := io.Copy(ioutil.Discard, file)
	if err != nil {
		return 0, nil, err
	}

	hasher := sha256.New()
	_, err = hasher.Write([]byte(header.Filename))
	if err != nil {
		return 0, nil, err
	}

	body, err := json.Marshal(&proxy.AddResponseMessage{
		Name: header.Filename,
		Hash: base64.URLEncoding.EncodeToString(hasher.Sum(nil)),
		Size: strconv.Itoa(int(header.Size)),
	})
	if err != nil {
		return 0, nil, err
	}

	_, err = w.Write(body)
	if err != nil {
		return 0, nil, err
	}

	return int(header.Size), hasher, nil
}
