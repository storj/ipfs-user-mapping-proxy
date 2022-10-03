package proxy_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/ipfs-user-mapping-proxy/db"
	proxydb "storj.io/ipfs-user-mapping-proxy/db"
	"storj.io/ipfs-user-mapping-proxy/mock"
	"storj.io/ipfs-user-mapping-proxy/proxy"
)

func TestPinLsHandler_MissingBasicAuth(t *testing.T) {
	runTest(t, nil, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *proxydb.DB) {
		// Add a record to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
		)
		require.NoError(t, err)

		req, err := pinLsRequest(server.URL+proxy.PinLsEndpoint, "")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

func TestPinLsHandler_InvalidQueryParams(t *testing.T) {
	runTest(t, nil, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add a record to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
		)
		require.NoError(t, err)

		// Pass an invalid query param.
		req, err := pinLsRequest(server.URL+proxy.PinLsEndpoint+"?stream", "john")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestPinLsHandle_NoPins(t *testing.T) {
	runTest(t, nil, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		req, err := pinLsRequest(server.URL+proxy.PinLsEndpoint, "john")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"Keys":{}}`, string(respBody))
	})
}

func TestPinLsHandle_Basic(t *testing.T) {
	runTest(t, nil, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add a record to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
		)
		require.NoError(t, err)

		req, err := pinLsRequest(server.URL+proxy.PinLsEndpoint, "john")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"Keys":{"pin-hash-1":{"Type":"recursive"}}}`, string(respBody))
	})
}

func TestPinLsHandle_RemovedPin(t *testing.T) {
	runTest(t, nil, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add a record to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
		)
		require.NoError(t, err)

		// Unpin the file.
		err = db.RemoveContentByHashForUser(ctx, "john", []string{"pin-hash-1"})
		require.NoError(t, err)

		req, err := pinLsRequest(server.URL+proxy.PinLsEndpoint, "john")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"Keys":{}}`, string(respBody))
	})
}

func TestPinLsHandle_MultiplePins(t *testing.T) {
	runTest(t, nil, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add a record to the database pinned by two different users.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
			proxydb.Content{User: "shawn", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
		)
		require.NoError(t, err)

		req, err := pinLsRequest(server.URL+proxy.PinLsEndpoint, "john")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"Keys":{"pin-hash-1":{"Type":"recursive"}}}`, string(respBody))
	})
}

func TestPinLsHandle_SomeoneElsePin(t *testing.T) {
	runTest(t, nil, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add a record to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
		)
		require.NoError(t, err)

		req, err := pinLsRequest(server.URL+proxy.PinLsEndpoint, "shawn")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"Keys":{}}`, string(respBody))

	})
}

func TestPinLsHandle_TwoOfThree(t *testing.T) {
	ipfsHandler := new(mock.IPFSPinRmHandler)
	runTest(t, ipfsHandler, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add some records to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
			proxydb.Content{User: "john", Hash: "pin-hash-2", Name: "second.jpg", Size: 1024},
			proxydb.Content{User: "john", Hash: "pin-hash-3", Name: "third.jpg", Size: 1024},
		)
		require.NoError(t, err)

		// Unpin the second file.
		err = db.RemoveContentByHashForUser(ctx, "john", []string{"pin-hash-2"})
		require.NoError(t, err)

		req, err := pinLsRequest(server.URL+proxy.PinLsEndpoint, "john")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Check that only the first and third files are in the list.
		respBody, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"Keys":{"pin-hash-1":{"Type":"recursive"},"pin-hash-3":{"Type":"recursive"}}}`, string(respBody))
	})
}

func TestPinLsHandle_MultiMix(t *testing.T) {
	runTest(t, new(mock.NoopHandler), func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add some records to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
			proxydb.Content{User: "shawn", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
			proxydb.Content{User: "john", Hash: "pin-hash-2", Name: "second.jpg", Size: 1024},
			proxydb.Content{User: "john", Hash: "pin-hash-3", Name: "third.jpg", Size: 1024},
			proxydb.Content{User: "shawn", Hash: "pin-hash-4", Name: "forth.jpg", Size: 1024},
		)
		require.NoError(t, err)

		req, err := pinLsRequest(server.URL+proxy.PinLsEndpoint, "john")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"Keys":{"pin-hash-1":{"Type":"recursive"},"pin-hash-2":{"Type":"recursive"},"pin-hash-3":{"Type":"recursive"}}}`, string(respBody))
	})
}

func pinLsRequest(url, user string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}

	if len(user) > 0 {
		req.SetBasicAuth(user, "somepassword")
	}

	return req, nil
}
