package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/ipfs-user-mapping-proxy/db"
)

// DAGImportResponseMessage is the JSON object returned to DAG Import requests.
type DAGImportResponseMessage struct {
	Root  *RootMeta       `json:",omitempty"`
	Stats *CarImportStats `json:",omitempty"`
}

// RootMeta is the metadata for a root pinning response.
type RootMeta struct {
	Cid         map[string]string
	PinErrorMsg string
}

// CarImportStats is the result stats of a DAG import request.
type CarImportStats struct {
	BlockCount      int64
	BlockBytesCount int64
}

// HandleDAGImport is an HTTP handler that intercepts
// the /api/v0/dag/import requests to the IPFS node.
//
// It retrieves the authenticated user from the requests and maps it to the
// imported content. The mapping is stored in the database.
func (p *Proxy) HandleDAGImport(w http.ResponseWriter, r *http.Request) {
	_ = p.handleDAGImport(r.Context(), w, r)
}

func (p *Proxy) handleDAGImport(ctx context.Context, w http.ResponseWriter, r *http.Request) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, _, ok := r.BasicAuth()
	if !ok {
		mon.Counter("dag_import_handler_response_codes", monkit.NewSeriesTag("code", strconv.Itoa(http.StatusUnauthorized))).Inc(1)
		p.log.Error("No basic auth in request")
		err = errors.New("no basic auth")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	for param := range r.URL.Query() {
		switch param {
		case "stats":
			if Stats(r) {
				continue
			}
			mon.Counter("dag_import_handler_invalid_query_param", monkit.NewSeriesTag("param", param)).Inc(1)
			p.log.Error("Invalid query param",
				zap.String("User", user),
				zap.String("Param", param))
			err = errors.New("stats argument cannot be false")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return err
		default:
			mon.Counter("dag_import_handler_invalid_query_param", monkit.NewSeriesTag("param", param)).Inc(1)
			p.log.Error("Invalid query param",
				zap.String("User", user),
				zap.String("Param", param))
			err = errors.New("only stats argument is allowed")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return err
		}
	}

	// Ensure that the stats param is set in the request
	if !Stats(r) {
		values := r.URL.Query()
		values.Set("stats", "true")
		r.URL.RawQuery = values.Encode()
	}

	wrapper := NewResponseWriterWrapper(w)
	p.proxy.ServeHTTP(wrapper, r)

	code := wrapper.StatusCode
	mon.Counter("dag_import_handler_response_codes", monkit.NewSeriesTag("code", strconv.Itoa(code))).Inc(1)

	if code != http.StatusOK {
		if code > 400 && code != http.StatusBadGateway {
			// BadGateway is logged by the proxy error handler
			p.log.Error("Proxy error",
				zap.String("User", user),
				zap.Int("Code", code),
				zap.ByteString("Body", wrapper.Body))
		}
		return fmt.Errorf("Proxy error (code %d)", code)
	}

	decoder := json.NewDecoder(strings.NewReader(string(wrapper.Body)))

	var cids []string
	for {
		var msg DAGImportResponseMessage
		err := decoder.Decode(&msg)
		if err == io.EOF {
			break
		}
		if err != nil {
			mon.Counter("dag_import_handler_error_unmarshal_response").Inc(1)
			p.log.Error("JSON response unmarshal error",
				zap.String("User", user),
				zap.ByteString("Body", wrapper.Body),
				zap.Error(err))
			return err
		}

		if msg.Root != nil {
			if msg.Root.PinErrorMsg != "" {
				mon.Counter("dag_import_handler_pin_error_msg").Inc(1)
				err = errors.New(msg.Root.PinErrorMsg)
				p.log.Error("DAG Import error",
					zap.String("User", user),
					zap.ByteString("Body", wrapper.Body),
					zap.Error(err))
				return err
			}

			cid, found := msg.Root.Cid["/"]
			if !found {
				mon.Counter("dag_import_handler_no_root_cid").Inc(1)
				err = errors.New("no root CID in response")
				p.log.Error("DAG Import error",
					zap.String("User", user),
					zap.ByteString("Body", wrapper.Body),
					zap.Error(err))
				return err
			}

			cids = append(cids, cid)
		}

		if msg.Stats != nil {
			// It is not ideal to set the total bytes count to each of the
			// imported CARs (in the case of multiple CARs in a single request),
			// but we have no better way to keep track of the uploaded size.
			size := msg.Stats.BlockBytesCount

			for _, cid := range cids {
				hash := cid
				name := cid + " (dag import)"
				err = p.db.Add(ctx, db.Content{
					User: user,
					Hash: hash,
					Name: name,
					Size: size,
				})
				if err != nil {
					mon.Counter("dag_import_handler_error_db_add").Inc(1)
					p.log.Error("Error adding content to database",
						zap.String("User", user),
						zap.String("Hash", hash),
						zap.String("Name", name),
						zap.Int64("Size", size),
						zap.Error(err))
					return err
				}
			}
			return nil
		}
	}

	return nil
}

func Stats(r *http.Request) bool {
	if !r.URL.Query().Has("stats") {
		return false
	}

	if r.URL.Query().Get("stats") == "false" {
		return false
	}

	return true
}
