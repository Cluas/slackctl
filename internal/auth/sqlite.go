package auth

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// QueryReadonlySQLite copies a SQLite DB (to avoid locking) and runs a query.
func QueryReadonlySQLite(dbPath, query string) ([]map[string]any, error) {
	// Copy the DB file to a temp location to avoid SQLITE_BUSY
	tmpDir, err := os.MkdirTemp("", "agent-slack-sqlite-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	copyPath := filepath.Join(tmpDir, "copy.sqlite")
	if err := copyFile(dbPath, copyPath); err != nil {
		return nil, fmt.Errorf("failed to copy SQLite DB: %w", err)
	}
	// Also copy WAL/SHM sidecars if present
	for _, ext := range []string{"-wal", "-shm"} {
		src := dbPath + ext
		if _, err := os.Stat(src); err == nil {
			_ = copyFile(src, copyPath+ext)
		}
	}

	db, err := sql.Open("sqlite", copyPath+"?mode=ro&_journal_mode=WAL")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any)
		for i, col := range cols {
			row[col] = values[i]
		}
		results = append(results, row)
	}
	return results, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
