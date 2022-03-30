package db

import (
	"context"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib" // registers pgx as a tagsql driver.
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/private/dbutil"
	_ "storj.io/private/dbutil/cockroachutil" // registers cockroach as a tagsql driver.
	"storj.io/private/migrate"
	"storj.io/private/tagsql"
)

// Error is the error class for datastore database.
var Error = errs.Class("db")

// DB is the database for mapping pinned IPFS content to users.
type DB struct {
	tagsql.DB
}

// Content represents a content record in the database.
type Content struct {
	// User is the user who uploaded the content.
	User string

	// Created is when the content was uploaded.
	Created time.Time

	// Hash is the IPFS hash of the uploaded content.
	Hash string

	// Name is the file name associated with the uploaded content.
	Name string

	// Size is the size in bytes of the uploaded content.
	Size int64
}

// Open creates instance of the database.
func Open(ctx context.Context, databaseURL string) (*DB, error) {
	_, _, impl, err := dbutil.SplitConnStr(databaseURL)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var driverName string
	switch impl {
	case dbutil.Postgres:
		driverName = "pgx"
	case dbutil.Cockroach:
		driverName = "cockroach"
	default:
		return nil, Error.New("unsupported implementation: %s", driverName)
	}

	db, err := tagsql.Open(ctx, driverName, databaseURL)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return Wrap(db), nil
}

// MigrateToLatest migrates pindb to the latest version.
func (db *DB) MigrateToLatest(ctx context.Context) error {
	err := db.Migration().Run(ctx, zap.NewExample())
	return Error.Wrap(err)
}

// Migration returns steps needed for migrating the database.
func (db *DB) Migration() *migrate.Migration {
	return &migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				DB:          &db.DB,
				Description: "Initial setup",
				Version:     0,
				Action: migrate.SQL{`
					CREATE TABLE IF NOT EXISTS content (
						id SERIAL PRIMARY KEY,
						username TEXT NOT NULL,
						created TIMESTAMP NOT NULL DEFAULT NOW(),
						hash TEXT UNIQUE NOT NULL,
						name TEXT NOT NULL,
						size BIGINT NOT NULL
					)
				`},
			},
		},
	}
}

// Add adds a content record to the database.
//
// The content's created time is ignored as it is automatically set by the database.
func (db *DB) Add(ctx context.Context, content Content) error {
	_, err := db.Exec(ctx, `
		INSERT INTO content (username, hash, name, size)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT(hash) DO NOTHING
	`, content.User, content.Hash, content.Name, content.Size)
	return Error.Wrap(err)
}

// List returns all content records from the database.
func (db *DB) List(ctx context.Context) ([]Content, error) {
	rows, err := db.Query(ctx, `
		SELECT username, created, hash, name, size
		FROM content
	`)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer rows.Close()

	var result []Content
	for rows.Next() {
		var content Content
		err := rows.Scan(&content.User, &content.Created, &content.Hash, &content.Name, &content.Size)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		result = append(result, content)
	}

	return result, nil
}

// Wrap turns a tagsql.DB into a DB struct.
func Wrap(db tagsql.DB) *DB {
	return &DB{DB: postgresRebind{DB: db}}
}

// This is needed for migrate to work.
// TODO: clean this up.
type postgresRebind struct{ tagsql.DB }

func (pq postgresRebind) Rebind(sql string) string {
	type sqlParseState int
	const (
		sqlParseStart sqlParseState = iota
		sqlParseInStringLiteral
		sqlParseInQuotedIdentifier
		sqlParseInComment
	)

	out := make([]byte, 0, len(sql)+10)

	j := 1
	state := sqlParseStart
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		switch state {
		case sqlParseStart:
			switch ch {
			case '?':
				out = append(out, '$')
				out = append(out, strconv.Itoa(j)...)
				state = sqlParseStart
				j++
				continue
			case '-':
				if i+1 < len(sql) && sql[i+1] == '-' {
					state = sqlParseInComment
				}
			case '"':
				state = sqlParseInQuotedIdentifier
			case '\'':
				state = sqlParseInStringLiteral
			}
		case sqlParseInStringLiteral:
			if ch == '\'' {
				state = sqlParseStart
			}
		case sqlParseInQuotedIdentifier:
			if ch == '"' {
				state = sqlParseStart
			}
		case sqlParseInComment:
			if ch == '\n' {
				state = sqlParseStart
			}
		}
		out = append(out, ch)
	}

	return string(out)
}
