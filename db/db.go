package db

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Content represents a content record in the database.
type Content struct {
	// User is the user who uploaded the content.
	User string

	// Hash is the IPFS hash of the uploaded content.
	Hash string

	// Name is the file name associated with the uploaded content.
	Name string

	// Size is the size in bytes of the uploaded content.
	Size int64
}

// Init creates the database schema, if not created yet.
func Init(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS content (
			id SERIAL PRIMARY KEY,
			username TEXT NOT NULL,
			hash TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			size BIGINT NOT NULL
		)
	`)
	return err
}

// Add adds a content record to the database.
func Add(ctx context.Context, db *pgxpool.Pool, content Content) error {
	_, err := db.Exec(ctx, `
		INSERT INTO content (username, hash, name, size)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT(hash) DO NOTHING
	`, content.User, content.Hash, content.Name, content.Size)
	return err
}

// List returns all content records from the database.
func List(ctx context.Context, db *pgxpool.Pool) ([]Content, error) {
	rows, err := db.Query(ctx, `
		SELECT username, hash, name, size
		FROM content
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Content
	for rows.Next() {
		var content Content
		err := rows.Scan(&content.User, &content.Hash, &content.Name, &content.Size)
		if err != nil {
			return nil, err
		}
		result = append(result, content)
	}

	return result, nil
}
