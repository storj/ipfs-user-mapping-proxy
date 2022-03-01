package proxy_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/kaloyan-raev/ipfs-user-mapping-proxy/db"
	"github.com/kaloyan-raev/ipfs-user-mapping-proxy/mock"
	"github.com/kaloyan-raev/ipfs-user-mapping-proxy/proxy"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"
	"github.com/zeebo/errs"

	"storj.io/common/testrand"
	"storj.io/private/dbutil/pgutil"
)

func TestAddHandler_MissingBasicAuth(t *testing.T) {
	runTest(t, mock.IPFSAddHandler, func(ctx context.Context, proxy *httptest.Server, dbPool *pgxpool.Pool) {
		req, err := addRequest(proxy.URL, "", "test.png", 1024)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

func TestAddHandler_InternalError(t *testing.T) {
	runTest(t, mock.ErrorHandler, func(ctx context.Context, proxy *httptest.Server, dbPool *pgxpool.Pool) {
		req, err := addRequest(proxy.URL, "test", "test.png", 1024)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestAddHandler(t *testing.T) {
	runTest(t, mock.IPFSAddHandler, func(ctx context.Context, proxy *httptest.Server, dbPool *pgxpool.Pool) {
		// Upload a file
		err := addFile(proxy.URL, "john", "first.jpg", 1024)
		require.NoError(t, err)

		// Check that the DB contains it
		contents, err := db.List(ctx, dbPool)
		require.NoError(t, err)
		require.Len(t, contents, 1)
		assert.Equal(t, "john", contents[0].User)
		assert.Equal(t, "first.jpg", contents[0].Name)
		assert.Equal(t, 1024, contents[0].Size)

		// Upload the same file
		err = addFile(proxy.URL, "john", "first.jpg", 1024)
		require.NoError(t, err)

		// Check that nothing changed in the DB
		contents, err = db.List(ctx, dbPool)
		require.NoError(t, err)
		require.Len(t, contents, 1)
		assert.Equal(t, "john", contents[0].User)
		assert.Equal(t, "first.jpg", contents[0].Name)
		assert.Equal(t, 1024, contents[0].Size)

		// Upload the same file, but by a different user
		err = addFile(proxy.URL, "shawn", "first.jpg", 1024)
		require.NoError(t, err)

		// Check that nothing changed in the DB, the file is still mapped to the first user
		contents, err = db.List(ctx, dbPool)
		require.NoError(t, err)
		require.Len(t, contents, 1)
		assert.Equal(t, "john", contents[0].User)
		assert.Equal(t, "first.jpg", contents[0].Name)
		assert.Equal(t, 1024, contents[0].Size)

		// Upload a different file with the second user
		err = addFile(proxy.URL, "shawn", "second.jpg", 1234)
		require.NoError(t, err)

		// Check that both users have one file each
		contents, err = db.List(ctx, dbPool)
		require.NoError(t, err)
		require.Len(t, contents, 2)
		assert.Equal(t, "john", contents[0].User)
		assert.Equal(t, "first.jpg", contents[0].Name)
		assert.Equal(t, 1024, contents[0].Size)
		assert.Equal(t, "shawn", contents[1].User)
		assert.Equal(t, "second.jpg", contents[1].Name)
		assert.Equal(t, 1234, contents[1].Size)

		// Upload a third file with the first user
		err = addFile(proxy.URL, "john", "third.jpg", 12987)
		require.NoError(t, err)

		// Check that both first user has two file, and the second user - one file
		contents, err = db.List(ctx, dbPool)
		require.NoError(t, err)
		require.Len(t, contents, 3)
		assert.Equal(t, "john", contents[0].User)
		assert.Equal(t, "first.jpg", contents[0].Name)
		assert.Equal(t, 1024, contents[0].Size)
		assert.Equal(t, "shawn", contents[1].User)
		assert.Equal(t, "second.jpg", contents[1].Name)
		assert.Equal(t, 1234, contents[1].Size)
		assert.Equal(t, "john", contents[2].User)
		assert.Equal(t, "third.jpg", contents[2].Name)
		assert.Equal(t, 12987, contents[2].Size)
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

func runTest(t *testing.T, mockHandler func(http.ResponseWriter, *http.Request), f func(context.Context, *httptest.Server, *pgxpool.Pool)) {
	ctx := context.Background()
	ipfsServer := httptest.NewServer(http.HandlerFunc(mockHandler))

	dbURI, err := initDB(ctx)
	require.NoError(t, err)

	ipfsServerURL, err := url.Parse(ipfsServer.URL)
	require.NoError(t, err)

	dbPool, err := pgxpool.Connect(ctx, dbURI)
	require.NoError(t, err)

	db.Init(ctx, dbPool)
	require.NoError(t, err)

	proxy := proxy.New("", ipfsServerURL, dbPool)
	tsProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.HandleAdd(w, r)
	}))

	f(ctx, tsProxy, dbPool)
}

func initDB(ctx context.Context) (dbURI string, err error) {
	dbURI, set := os.LookupEnv("STORJ_TEST_POSTGRES")
	if !set {
		return "", errors.New("skipping test suite; STORJ_TEST_POSTGRES is not set.")
	}

	conn, err := pgx.Connect(ctx, dbURI)
	if err != nil {
		return "", err
	}
	defer func() {
		err = errs.Combine(err, conn.Close(ctx))
	}()

	schemaName := pgutil.CreateRandomTestingSchemaName(6)
	_, err = conn.Exec(ctx, "CREATE SCHEMA "+pgutil.QuoteSchema(schemaName))
	if err != nil {
		return "", err
	}

	return pgutil.ConnstrWithSchema(dbURI, schemaName), nil
}
