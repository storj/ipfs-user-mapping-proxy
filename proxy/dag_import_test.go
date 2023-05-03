package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/ipfs-user-mapping-proxy/db"
	"storj.io/ipfs-user-mapping-proxy/mock"
	"storj.io/ipfs-user-mapping-proxy/proxy"
)

func TestDAGImportHandler_MissingBasicAuth(t *testing.T) {
	runTest(t, new(mock.IPFSDAGImportHandler), func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		req, err := addRequest(server.URL+proxy.DAGImportEndpoint, "", 1024, "test.car")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		// Check that DB is still empty
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Empty(t, contents)
	})
}

func TestDAGImportHandler_InternalError(t *testing.T) {
	runTest(t, new(mock.ErrorHandler), func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		req, err := addRequest(server.URL+proxy.DAGImportEndpoint, "test", 1024, "test.car")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		// Check that DB is still empty
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Empty(t, contents)
	})
}

func TestDAGImportHandler_InvalidQueryParams(t *testing.T) {
	runTest(t, new(mock.IPFSDAGImportHandler), func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Pass an invalid query param
		req, err := addRequest(server.URL+proxy.DAGImportEndpoint+"?silent", "test", 1024, "test.car")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		// Check that DB is still empty
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Empty(t, contents)
	})
}

func TestDAGImportHandler_Stats(t *testing.T) {
	runTest(t, new(mock.IPFSDAGImportHandler), func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		err := addFile(server.URL+proxy.DAGImportEndpoint+"?stats", "test", 1024, "test.car")
		require.NoError(t, err)

		// Check that the DB contains the wrapping directory
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 1)
		assert.Equal(t, "test", contents[0].User)
		assert.Equal(t, mock.Hash("test.car"), contents[0].Hash)
		assert.Equal(t, mock.Hash("test.car")+" (dag import)", contents[0].Name)
		assert.InDelta(t, 1024, contents[0].Size, 20)
		assert.WithinDuration(t, time.Now(), contents[0].Created, 1*time.Minute)
		assert.Nil(t, contents[0].Removed)
	})
}

func TestDAGImportHandler_StatsTrue(t *testing.T) {
	runTest(t, new(mock.IPFSDAGImportHandler), func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		err := addFile(server.URL+proxy.DAGImportEndpoint+"?stats=true", "test", 1024, "test.car")
		require.NoError(t, err)

		// Check that the DB contains the wrapping directory
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 1)
		assert.Equal(t, "test", contents[0].User)
		assert.Equal(t, mock.Hash("test.car"), contents[0].Hash)
		assert.Equal(t, mock.Hash("test.car")+" (dag import)", contents[0].Name)
		assert.InDelta(t, 1024, contents[0].Size, 20)
		assert.WithinDuration(t, time.Now(), contents[0].Created, 1*time.Minute)
		assert.Nil(t, contents[0].Removed)
	})
}

func TestDAGImportHandler_StatsFalse(t *testing.T) {
	runTest(t, new(mock.IPFSDAGImportHandler), func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		req, err := addRequest(server.URL+proxy.DAGImportEndpoint+"?stats=false", "test", 1024, "test.car")
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		// Check that DB is still empty
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Empty(t, contents)
	})
}

func TestDAGImportHandler_Basic(t *testing.T) {
	runTest(t, new(mock.IPFSDAGImportHandler), func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		// Import a CAR file
		err := addFile(server.URL+proxy.DAGImportEndpoint, "john", 1024, "first.car")
		require.NoError(t, err)

		// Check that the DB contains it
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 1)

		content1 := contents[0]
		assert.Equal(t, "john", content1.User)
		assert.Equal(t, mock.Hash("first.car"), content1.Hash)
		assert.Equal(t, mock.Hash("first.car")+" (dag import)", content1.Name)
		assert.InDelta(t, 1024, content1.Size, 20)
		assert.WithinDuration(t, time.Now(), content1.Created, 1*time.Minute)
		assert.Nil(t, content1.Removed)

		// Upload the same CAR file
		err = addFile(server.URL+proxy.DAGImportEndpoint, "john", 1024, "first.car")
		require.NoError(t, err)

		// Check that nothing changed in the DB
		contents, err = db.ListAll(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 1)
		assert.Equal(t, content1, contents[0])

		// Upload the same file, but by a different user
		err = addFile(server.URL+proxy.DAGImportEndpoint, "shawn", 1024, "first.car")
		require.NoError(t, err)

		// Check that both users have the same file
		contents, err = db.ListAll(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 2)
		assert.Equal(t, content1, contents[0])
		assert.Equal(t, "shawn", contents[1].User)
		assert.Equal(t, content1.Hash, contents[1].Hash)
		assert.Equal(t, content1.Name, contents[1].Name)
		assert.Equal(t, content1.Size, contents[1].Size)

		// Upload a different file with the second user
		err = addFile(server.URL+proxy.DAGImportEndpoint, "shawn", 1234, "second.car")
		require.NoError(t, err)

		// Check that the first user has one file, and the second - two files
		contents, err = db.ListAll(ctx)
		require.NoError(t, err)

		sortByCreated(contents)
		require.Len(t, contents, 3)
		assert.Equal(t, content1, contents[0])
		assert.Equal(t, "shawn", contents[1].User)
		assert.Equal(t, content1.Hash, contents[1].Hash)
		assert.Equal(t, content1.Name, contents[1].Name)
		assert.Equal(t, content1.Size, contents[1].Size)
		assert.Equal(t, "shawn", contents[2].User)
		assert.Equal(t, mock.Hash("second.car"), contents[2].Hash)
		assert.Equal(t, mock.Hash("second.car")+" (dag import)", contents[2].Name)
		assert.InDelta(t, 1234, contents[2].Size, 20)

		// Upload a third file with the first user
		err = addFile(server.URL+proxy.DAGImportEndpoint, "john", 12987, "third.car")
		require.NoError(t, err)

		// Check that both users have two files
		contents, err = db.ListAll(ctx)
		require.NoError(t, err)

		sortByCreated(contents)
		require.Len(t, contents, 4)
		assert.Equal(t, content1, contents[0])
		assert.Equal(t, "shawn", contents[1].User)
		assert.Equal(t, content1.Hash, contents[1].Hash)
		assert.Equal(t, content1.Name, contents[1].Name)
		assert.Equal(t, content1.Size, contents[1].Size)
		assert.Equal(t, "shawn", contents[2].User)
		assert.Equal(t, mock.Hash("second.car"), contents[2].Hash)
		assert.Equal(t, mock.Hash("second.car")+" (dag import)", contents[2].Name)
		assert.InDelta(t, 1234, contents[2].Size, 20)
		assert.Equal(t, "john", contents[3].User)
		assert.Equal(t, mock.Hash("third.car"), contents[3].Hash)
		assert.Equal(t, mock.Hash("third.car")+" (dag import)", contents[3].Name)
		assert.InDelta(t, 12987, contents[3].Size, 20)
	})
}

func TestDAGImportHandler_PinErrorMsg(t *testing.T) {
	runTest(t, new(mock.IPFSDAGImportErrorHandler), func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		err := addFile(server.URL+proxy.DAGImportEndpoint, "test", 1024, "test.car")
		require.NoError(t, err)

		// Check that DB is still empty
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Empty(t, contents)
	})
}

func TestDAGImportHandler_NoRootCID(t *testing.T) {
	runTest(t, new(mock.IPFSDAGImportNoRootHandler), func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		err := addFile(server.URL+proxy.DAGImportEndpoint, "test", 1024, "test.car")
		require.NoError(t, err)

		// Check that DB is still empty
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)
		require.Empty(t, contents)
	})
}

func TestDAGImportHandler_MultipleFiles(t *testing.T) {
	runTest(t, new(mock.IPFSDAGImportHandler), func(t *testing.T, ctx *testcontext.Context, server *httptest.Server, db *db.DB) {
		err := addFile(server.URL+proxy.DAGImportEndpoint, "test", 1024, "test.car", "test2.car")
		require.NoError(t, err)

		// Check that the DB contains both
		contents, err := db.ListAll(ctx)
		require.NoError(t, err)

		sortByCreated(contents)
		require.Len(t, contents, 2)
		assert.Equal(t, "test", contents[0].User)
		assert.Equal(t, mock.Hash("test.car"), contents[0].Hash)
		assert.Equal(t, mock.Hash("test.car")+" (dag import)", contents[0].Name)
		assert.InDelta(t, 2048, contents[0].Size, 20)
		assert.WithinDuration(t, time.Now(), contents[0].Created, 1*time.Minute)
		assert.Nil(t, contents[1].Removed)
		assert.Equal(t, "test", contents[1].User)
		assert.Equal(t, mock.Hash("test2.car"), contents[1].Hash)
		assert.Equal(t, mock.Hash("test2.car")+" (dag import)", contents[1].Name)
		assert.InDelta(t, 2048, contents[1].Size, 20)
		assert.WithinDuration(t, time.Now(), contents[1].Created, 1*time.Minute)
		assert.Nil(t, contents[1].Removed)
	})
}
