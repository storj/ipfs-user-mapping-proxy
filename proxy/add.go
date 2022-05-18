package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/ipfs-user-mapping-proxy/db"
)

// AddResponseMessage is the JSON object returned to Add requests.
type AddResponseMessage struct {
	Name string `json:"Name"`
	Hash string `json:"Hash"`
	Size string `json:"Size"`
}

// HandleAdd is an HTTP handler that intercepts
// the /api/v0/add requests to the IPFS node.
//
// It retrieves the authenticated user from the requests and maps it to the
// uploaded content. The mapping is stored in the database.
func (p *Proxy) HandleAdd(w http.ResponseWriter, r *http.Request) {
	_ = p.handleAdd(r.Context(), w, r)
}

func (p *Proxy) handleAdd(ctx context.Context, w http.ResponseWriter, r *http.Request) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, _, ok := r.BasicAuth()
	if !ok {
		mon.Counter("add_handler_response_codes", monkit.NewSeriesTag("code", strconv.Itoa(http.StatusUnauthorized))).Inc(1)
		p.log.Error("No basic auth in request")
		err = errors.New("no basic auth")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	for param := range r.URL.Query() {
		switch param {
		case "wrap-with-directory":
			continue
		default:
			mon.Counter("add_handler_invalid_query_param", monkit.NewSeriesTag("param", param)).Inc(1)
			p.log.Error("Invalid query param",
				zap.String("User", user),
				zap.String("Param", param))
			err = errors.New("only wrap-with-directory argument is allowed")
			http.Error(w, err.Error(), http.StatusForbidden)
			return err
		}
	}

	wrapper := NewResponseWriterWrapper(w)
	p.proxy.ServeHTTP(wrapper, r)

	code := wrapper.StatusCode
	mon.Counter("add_handler_response_codes", monkit.NewSeriesTag("code", strconv.Itoa(code))).Inc(1)

	if code != http.StatusOK {
		if code > 400 && code != http.StatusBadGateway {
			// BadGateway is logged by the proxy error handler
			p.log.Error("Proxy error",
				zap.String("User", user),
				zap.Int("Code", code),
				zap.ByteString("Body", wrapper.Body))
		}
		return err
	}

	decoder := json.NewDecoder(strings.NewReader(string(wrapper.Body)))
	var messages []AddResponseMessage
	for {
		var msg AddResponseMessage
		err := decoder.Decode(&msg)
		if err == io.EOF {
			break
		}
		if err != nil {
			mon.Counter("error_unmarshal_response").Inc(1)
			p.log.Error("JSON response unmarshal error",
				zap.String("User", user),
				zap.ByteString("Body", wrapper.Body),
				zap.Error(err))
			return err
		}
		messages = append(messages, msg)
	}

	if len(messages) == 0 {
		mon.Counter("error_no_response_message").Inc(1)
		p.log.Error("No response message",
			zap.String("User", user),
			zap.ByteString("Body", wrapper.Body),
			zap.Error(err))
		return errors.New("no response message")
	}

	name := messages[len(messages)-1].Name
	if WrapWithDirectory(r) {
		name = messages[0].Name + " (wrapped)"
	}

	hash := messages[len(messages)-1].Hash

	size, err := strconv.ParseInt(messages[len(messages)-1].Size, 10, 64)
	if err != nil {
		mon.Counter("error_parse_size").Inc(1)
		p.log.Error("Size parse error",
			zap.String("User", user),
			zap.String("Size", messages[len(messages)-1].Size), zap.Error(err))
		return err
	}

	err = p.db.Add(ctx, db.Content{
		User: user,
		Hash: hash,
		Name: name,
		Size: size,
	})
	if err != nil {
		mon.Counter("error_db_add").Inc(1)
		p.log.Error("Error adding content to database",
			zap.String("User", user),
			zap.String("Hash", hash),
			zap.String("Name", name),
			zap.Int64("Size", size),
			zap.Error(err))
		return err
	}

	return nil
}

func WrapWithDirectory(r *http.Request) bool {
	if !r.URL.Query().Has("wrap-with-directory") {
		return false
	}

	if r.URL.Query().Get("wrap-with-directory") == "false" {
		return false
	}

	return true
}
