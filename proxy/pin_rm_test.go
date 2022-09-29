package proxy_test

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/ipfs-user-mapping-proxy/db"
	proxydb "storj.io/ipfs-user-mapping-proxy/db"
	"storj.io/ipfs-user-mapping-proxy/mock"
	"storj.io/ipfs-user-mapping-proxy/proxy"
)

func TestPinRmHandler_MissingBasicAuth(t *testing.T) {
	ipfsBackend := mock.IPFSPinRmHandler{}
	runTest(t, ipfsBackend.ServeHTTP, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *proxydb.DB) {
		// Add a record to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
		)
		require.NoError(t, err)

		req, err := pinRmRequest(server.URL+proxy.PinRmEndpoint, "", "pin-hash-1")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		// Check that the DB record was not marked as removed.
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 1)
		assert.Nil(t, contents[0].Removed)

		// Check that the IPFS backend was not invoked.
		assert.False(t, ipfsBackend.Invoked)
	})
}

func TestPinRmHandler_InternalError(t *testing.T) {
	runTest(t, mock.ErrorHandler, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *proxydb.DB) {
		// Add a record to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
		)
		require.NoError(t, err)

		req, err := pinRmRequest(server.URL+proxy.PinRmEndpoint, "john", "pin-hash-1")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		// Although the IPFS backend erred, we still mark the content as removed in the proxy DB.
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 1)
		require.NotNil(t, contents[0].Removed)
		assert.WithinDuration(t, time.Now(), *contents[0].Removed, 1*time.Minute)
	})
}

func TestPinRmHandler_InvalidQueryParams(t *testing.T) {
	ipfsBackend := mock.IPFSPinRmHandler{}
	runTest(t, ipfsBackend.ServeHTTP, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add a record to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
		)
		require.NoError(t, err)

		// Pass an invalid query param.
		req, err := pinRmRequest(server.URL+proxy.PinRmEndpoint+"?recursive", "john")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		// Check that the DB record was not marked as removed.
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 1)
		assert.Nil(t, contents[0].Removed)

		// Check that the IPFS backend was not invoked.
		assert.False(t, ipfsBackend.Invoked)
	})
}

func TestPinRmHandler_NoArgs(t *testing.T) {
	ipfsBackend := mock.IPFSPinRmHandler{}
	runTest(t, ipfsBackend.ServeHTTP, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add a record to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
		)
		require.NoError(t, err)

		// Do not pass any hashes to remove.
		req, err := pinRmRequest(server.URL+proxy.PinRmEndpoint, "john")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		// Check that the DB record was not marked as removed.
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 1)
		assert.Nil(t, contents[0].Removed)

		// Check that the IPFS backend was not invoked.
		assert.False(t, ipfsBackend.Invoked)
	})
}

func TestPinRmHandle_Basic(t *testing.T) {
	ipfsBackend := mock.IPFSPinRmHandler{}
	runTest(t, ipfsBackend.ServeHTTP, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add a record to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
		)
		require.NoError(t, err)

		req, err := pinRmRequest(server.URL+proxy.PinRmEndpoint, "john", "pin-hash-1")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"Pins":["pin-hash-1"]}`, string(respBody))

		// Check that the content is marked as removed in the proxy DB.
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 1)
		require.NotNil(t, contents[0].Removed)
		assert.WithinDuration(t, time.Now(), *contents[0].Removed, 1*time.Minute)

		// Check that the IPFS backend unpinned the content.
		assert.True(t, ipfsBackend.Invoked)
		assert.Equal(t, []string{"pin-hash-1"}, ipfsBackend.Removed)
	})
}

func TestPinRmHandle_MultiplePins(t *testing.T) {
	ipfsBackend := mock.IPFSPinRmHandler{}
	runTest(t, ipfsBackend.ServeHTTP, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add a record to the database pinned by two different users.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
			proxydb.Content{User: "shawn", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
		)
		require.NoError(t, err)

		req, err := pinRmRequest(server.URL+proxy.PinRmEndpoint, "john", "pin-hash-1")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"Pins":["pin-hash-1"]}`, string(respBody))

		// Check that the content is marked as removed only for john, but not for shawn.
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)

		sortByCreated(contents)
		require.Len(t, contents, 2)
		assert.Equal(t, "john", contents[0].User)
		require.NotNil(t, contents[0].Removed)
		assert.WithinDuration(t, time.Now(), *contents[0].Removed, 1*time.Minute)
		assert.Equal(t, "shawn", contents[1].User)
		assert.Nil(t, contents[1].Removed)

		// Check that the IPFS backend was not invoked - the content must be still pinned for shawn.
		assert.False(t, ipfsBackend.Invoked)
	})
}

func TestPinRmHandle_NonExistingPin(t *testing.T) {
	ipfsBackend := mock.IPFSPinRmHandler{}
	runTest(t, ipfsBackend.ServeHTTP, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add a record to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
		)
		require.NoError(t, err)

		// Try removing content pinned by someone else.
		req, err := pinRmRequest(server.URL+proxy.PinRmEndpoint, "john", "pin-hash-2")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		// Check that the DB record was not marked as removed.
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 1)
		assert.Nil(t, contents[0].Removed)

		// Check that the IPFS backend was not invoked.
		assert.False(t, ipfsBackend.Invoked)
	})
}

func TestPinRmHandle_SomeoneElsePin(t *testing.T) {
	ipfsBackend := mock.IPFSPinRmHandler{}
	runTest(t, ipfsBackend.ServeHTTP, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add a record to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
		)
		require.NoError(t, err)

		// Try removing content pinned by someone else.
		req, err := pinRmRequest(server.URL+proxy.PinRmEndpoint, "shawn", "pin-hash-1")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		// Check that the DB record was not marked as removed.
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 1)
		assert.Nil(t, contents[0].Removed)

		// Check that the IPFS backend was not invoked.
		assert.False(t, ipfsBackend.Invoked)
	})
}

func TestPinRmHandle_TwoOfThree(t *testing.T) {
	ipfsBackend := mock.IPFSPinRmHandler{}
	runTest(t, ipfsBackend.ServeHTTP, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add some records to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
			proxydb.Content{User: "john", Hash: "pin-hash-2", Name: "second.jpg", Size: 1024},
			proxydb.Content{User: "john", Hash: "pin-hash-3", Name: "third.jpg", Size: 1024},
		)
		require.NoError(t, err)

		// Unpin only the first and third files.
		req, err := pinRmRequest(server.URL+proxy.PinRmEndpoint, "john", "pin-hash-1", "pin-hash-3")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"Pins":["pin-hash-1", "pin-hash-3"]}`, string(respBody))

		// Check that only the first and third files are marked as removed.
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)

		sortByCreated(contents)
		require.Len(t, contents, 3)
		assert.Equal(t, "pin-hash-1", contents[0].Hash)
		require.NotNil(t, contents[0].Removed)
		assert.WithinDuration(t, time.Now(), *contents[0].Removed, 1*time.Minute)
		assert.Equal(t, "pin-hash-2", contents[1].Hash)
		assert.Nil(t, contents[1].Removed)
		assert.Equal(t, "pin-hash-3", contents[2].Hash)
		require.NotNil(t, contents[2].Removed)
		assert.WithinDuration(t, time.Now(), *contents[2].Removed, 1*time.Minute)

		// Check that the IPFS backend unpinned the two files.
		assert.True(t, ipfsBackend.Invoked)
		assert.Equal(t, []string{"pin-hash-1", "pin-hash-3"}, ipfsBackend.Removed)
	})
}

func TestPinRmHandle_OneExistsAndOneNot(t *testing.T) {
	ipfsBackend := mock.IPFSPinRmHandler{}
	runTest(t, ipfsBackend.ServeHTTP, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add some records to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
			proxydb.Content{User: "john", Hash: "pin-hash-2", Name: "second.jpg", Size: 1024},
		)
		require.NoError(t, err)

		// Unpin only the first and a non-existing file.
		req, err := pinRmRequest(server.URL+proxy.PinRmEndpoint, "john", "pin-hash-1", "pin-hash-3")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		// Check that the DB record was not marked as removed.
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 2)
		assert.Nil(t, contents[0].Removed)
		assert.Nil(t, contents[1].Removed)

		// Check that the IPFS backend was not invoked.
		assert.False(t, ipfsBackend.Invoked)
	})
}

func TestPinRmHandle_MultiMix(t *testing.T) {
	ipfsBackend := mock.IPFSPinRmHandler{}
	runTest(t, ipfsBackend.ServeHTTP, func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Add some records to the database.
		err := prefillDB(ctx, db,
			proxydb.Content{User: "john", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
			proxydb.Content{User: "shawn", Hash: "pin-hash-1", Name: "first.jpg", Size: 1024},
			proxydb.Content{User: "john", Hash: "pin-hash-2", Name: "second.jpg", Size: 1024},
			proxydb.Content{User: "john", Hash: "pin-hash-3", Name: "third.jpg", Size: 1024},
			proxydb.Content{User: "shawn", Hash: "pin-hash-4", Name: "forth.jpg", Size: 1024},
		)
		require.NoError(t, err)

		// Unpin all john's files.
		req, err := pinRmRequest(server.URL+proxy.PinRmEndpoint, "john", "pin-hash-1", "pin-hash-2", "pin-hash-3")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"Pins":["pin-hash-1", "pin-hash-2", "pin-hash-3"]}`, string(respBody))

		// Check that the second and third file were marked as removed.
		// The first file is also pinned by shawn, so it should not be marked as removed.
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)

		sortByCreated(contents)
		require.Len(t, contents, 5)
		assert.Equal(t, "pin-hash-1", contents[0].Hash)
		assert.Equal(t, "john", contents[0].User)
		require.NotNil(t, contents[0].Removed)
		assert.WithinDuration(t, time.Now(), *contents[0].Removed, 1*time.Minute)
		assert.Equal(t, "pin-hash-1", contents[1].Hash)
		assert.Equal(t, "shawn", contents[1].User)
		assert.Nil(t, contents[1].Removed)
		assert.Equal(t, "pin-hash-2", contents[2].Hash)
		assert.Equal(t, "john", contents[2].User)
		require.NotNil(t, contents[2].Removed)
		assert.WithinDuration(t, time.Now(), *contents[2].Removed, 1*time.Minute)
		assert.Equal(t, "pin-hash-3", contents[3].Hash)
		assert.Equal(t, "john", contents[3].User)
		require.NotNil(t, contents[3].Removed)
		assert.WithinDuration(t, time.Now(), *contents[3].Removed, 1*time.Minute)
		assert.Equal(t, "pin-hash-4", contents[4].Hash)
		assert.Equal(t, "shawn", contents[4].User)
		assert.Nil(t, contents[4].Removed)

		// Check that the IPFS backend was invoked only for the second and third file, but not for the first file.
		assert.True(t, ipfsBackend.Invoked)
		assert.Equal(t, []string{"pin-hash-2", "pin-hash-3"}, ipfsBackend.Removed)
	})
}

func prefillDB(ctx context.Context, db *proxydb.DB, contents ...proxydb.Content) error {
	for _, content := range contents {
		err := db.Add(ctx, content)
		if err != nil {
			return err
		}
	}
	return nil
}

func pinRmRequest(url, user string, hashes ...string) (*http.Request, error) {
	if len(hashes) > 0 {
		url += "?arg=" + strings.Join(hashes, "&arg=")
	}

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}

	if len(user) > 0 {
		req.SetBasicAuth(user, "somepassword")
	}

	return req, nil
}
