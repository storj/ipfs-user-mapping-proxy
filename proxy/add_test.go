package proxy_test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/ipfs-user-mapping-proxy/db"
	"storj.io/ipfs-user-mapping-proxy/mock"
	"storj.io/ipfs-user-mapping-proxy/proxy"
	"storj.io/private/dbutil"
	"storj.io/private/dbutil/tempdb"
)

func TestAddHandler_MissingBasicAuth(t *testing.T) {
	runTest(t, mock.IPFSAddHandler, func(t *testing.T, ctx testcontext.Context, proxy *httptest.Server, db *db.DB) {
		req, err := addRequest(proxy.URL, "", "test.png", 1024)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		// Check that DB is still empty
		contents, err := db.List(ctx)
		require.NoError(t, err)
		require.Empty(t, contents)
	})
}

func TestAddHandler_InternalError(t *testing.T) {
	runTest(t, mock.ErrorHandler, func(t *testing.T, ctx testcontext.Context, proxy *httptest.Server, db *db.DB) {
		req, err := addRequest(proxy.URL, "test", "test.png", 1024)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		// Check that DB is still empty
		contents, err := db.List(ctx)
		require.NoError(t, err)
		require.Empty(t, contents)
	})
}

func TestAddHandler(t *testing.T) {
	runTest(t, mock.IPFSAddHandler, func(t *testing.T, ctx testcontext.Context, proxy *httptest.Server, db *db.DB) {
		// Upload a file
		err := addFile(proxy.URL, "john", "first.jpg", 1024)
		require.NoError(t, err)

		// Check that the DB contains it
		contents, err := db.List(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 1)

		content1 := contents[0]
		assert.Equal(t, "john", content1.User)
		assert.Equal(t, "first.jpg", content1.Name)
		assert.Equal(t, int64(1024), content1.Size)
		assert.WithinDuration(t, time.Now(), content1.Created, 1*time.Minute)

		// Upload the same file
		err = addFile(proxy.URL, "john", "first.jpg", 1024)
		require.NoError(t, err)

		// Check that nothing changed in the DB
		contents, err = db.List(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 1)
		assert.Equal(t, content1, contents[0])

		// Upload the same file, but by a different user
		err = addFile(proxy.URL, "shawn", "first.jpg", 1024)
		require.NoError(t, err)

		// Check that both users have the same file
		contents, err = db.List(ctx)
		require.NoError(t, err)
		require.Len(t, contents, 2)
		assert.Equal(t, content1, contents[0])
		assert.Equal(t, "shawn", contents[1].User)
		assert.Equal(t, content1.Name, contents[1].Name)
		assert.Equal(t, content1.Size, contents[1].Size)

		// Upload a different file with the second user
		err = addFile(proxy.URL, "shawn", "second.jpg", 1234)
		require.NoError(t, err)

		// Check that the first user has one file, and the second - two files
		contents, err = db.List(ctx)
		sortByCreated(contents)
		require.NoError(t, err)
		require.Len(t, contents, 3)
		assert.Equal(t, content1, contents[0])
		assert.Equal(t, "shawn", contents[1].User)
		assert.Equal(t, content1.Name, contents[1].Name)
		assert.Equal(t, content1.Size, contents[1].Size)
		assert.Equal(t, "shawn", contents[2].User)
		assert.Equal(t, "second.jpg", contents[2].Name)
		assert.Equal(t, int64(1234), contents[2].Size)

		// Upload a third file with the first user
		err = addFile(proxy.URL, "john", "third.jpg", 12987)
		require.NoError(t, err)

		// Check that both users have two files
		contents, err = db.List(ctx)
		sortByCreated(contents)
		require.NoError(t, err)
		require.Len(t, contents, 4)
		assert.Equal(t, content1, contents[0])
		assert.Equal(t, "shawn", contents[1].User)
		assert.Equal(t, content1.Name, contents[1].Name)
		assert.Equal(t, content1.Size, contents[1].Size)
		assert.Equal(t, "shawn", contents[2].User)
		assert.Equal(t, "second.jpg", contents[2].Name)
		assert.Equal(t, int64(1234), contents[2].Size)
		assert.Equal(t, "john", contents[3].User)
		assert.Equal(t, "third.jpg", contents[3].Name)
		assert.Equal(t, int64(12987), contents[3].Size)
	})
}

func addFile(url, user, fileName string, fileSize int) error {
	req, err := addRequest(url, user, fileName, fileSize)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status code: expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func addRequest(url, user, fileName string, fileSize int) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, err
	}

	fw.Write(testrand.BytesInt(fileSize))
	if err != nil {
		return nil, err
	}

	writer.Close()

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body.Bytes()))
	if err != nil {
		return nil, err
	}

	if len(user) > 0 {
		req.SetBasicAuth(user, "somepassword")
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req, nil
}

func sortByCreated(contents []db.Content) {
	sort.Slice(contents, func(i, j int) bool {
		return contents[i].Created.Before(contents[j].Created)
	})
}

func runTest(t *testing.T, mockHandler func(http.ResponseWriter, *http.Request), f func(*testing.T, testcontext.Context, *httptest.Server, *db.DB)) {
	for _, impl := range []dbutil.Implementation{dbutil.Postgres, dbutil.Cockroach} {
		impl := impl
		t.Run(strings.Title(impl.String()), func(t *testing.T) {
			t.Parallel()

			ctx := testcontext.New(t)

			ipfsServer := httptest.NewServer(http.HandlerFunc(mockHandler))

			dbURI := dbURI(t, impl)

			ipfsServerURL, err := url.Parse(ipfsServer.URL)
			require.NoError(t, err)

			tempDB, err := tempdb.OpenUnique(ctx, dbURI, "ipfs-user-mapping-proxy")
			require.NoError(t, err)
			defer ctx.Check(tempDB.Close)

			log, err := zap.NewDevelopment()
			require.NoError(t, err)

			db := db.Wrap(tempDB.DB).WithLog(log)

			err = db.MigrateToLatest(ctx)
			require.NoError(t, err)

			proxy := proxy.New(log, db, "", ipfsServerURL)
			tsProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				proxy.HandleAdd(w, r)
			}))

			f(t, ctx, tsProxy, db)
		})
	}
}

func dbURI(t *testing.T, impl dbutil.Implementation) (dbURI string) {
	switch impl {
	case dbutil.Postgres:
		dbURI, set := os.LookupEnv("STORJ_TEST_POSTGRES")
		if !set {
			t.Skip("skipping test suite; STORJ_TEST_POSTGRES is not set.")
		}
		return dbURI
	case dbutil.Cockroach:
		dbURI, set := os.LookupEnv("STORJ_TEST_COCKROACH")
		if !set {
			t.Skip("skipping test suite; STORJ_TEST_COCKROACH is not set.")
		}
		return dbURI
	default:
		t.Errorf("unsupported database implementation %q", impl)
		return ""
	}
}
