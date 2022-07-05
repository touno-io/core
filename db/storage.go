package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Storage interface that is implemented by storage providers
type Storage struct {
	stx        *PGTx
	gcInterval time.Duration
	done       chan struct{}

	sqlSelect string
	sqlInsert string
	sqlDelete string
	sqlReset  string
	sqlGC     string
}

var (
	dropQuery = `DROP TABLE IF EXISTS "cache"."%s";`
	initQuery = []string{
		`CREATE SCHEMA IF NOT EXISTS "cache";`,
		`CREATE TABLE IF NOT EXISTS "cache"."%s" (
			s_key  VARCHAR(64) PRIMARY KEY NOT NULL DEFAULT '',
			a_value  BYTEA NOT NULL,
			t_expire  BIGINT NOT NULL DEFAULT '0'
		);`,
		`CREATE INDEX IF NOT EXISTS "idx_expire" ON "cache"."%s" (t_expire);`,
	}
	checkSchemaQuery = `SELECT DATA_TYPE FROM INFORMATION_SCHEMA.COLUMNS
		WHERE table_name = '%s' AND COLUMN_NAME = 'v';`
)

// New creates a new storage
func CacheNew(pgx *PGClient, tableName string) *Storage {
	stx, err := pgx.Begin(LevelReadUncommitted)
	if IsRollback(err, stx) {
		return nil
	}

	return &Storage{
		stx:        stx,
		gcInterval: 10 * time.Second,
		done:       make(chan struct{}),
		sqlSelect:  fmt.Sprintf(`SELECT a_value, t_expire FROM "cache"."%s" WHERE s_key=$1;`, tableName),
		sqlInsert:  fmt.Sprintf(`INSERT INTO "cache"."%s" (s_key, a_value, t_expire) VALUES ($1, $2, $3) ON CONFLICT (s_key) DO UPDATE SET a_value = $2, t_expire = $3`, tableName),
		sqlDelete:  fmt.Sprintf(`DELETE FROM "cache"."%s" WHERE s_key=$1`, tableName),
		sqlReset:   fmt.Sprintf(`TRUNCATE TABLE "cache"."%s";`, tableName),
		sqlGC:      fmt.Sprintf(`DELETE FROM "cache"."%s" WHERE t_expire <= $1 AND t_expire != 0`, tableName),
	}
}

// Get value by key
func (s *Storage) Get(key string) ([]byte, error) {
	if len(key) <= 0 {
		return nil, nil
	}
	row, err := s.stx.QueryOne(s.sqlSelect, key)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Add db response to data
	data := row.ToByte("a_value")
	exp := row.ToInt64("t_expire")

	// If the expiration time has already passed, then return nil
	if exp != 0 && exp <= time.Now().Unix() {
		return nil, nil
	}

	return data, nil
}

// Set key with value
func (s *Storage) Set(key string, val []byte, exp time.Duration) error {
	// Ain't Nobody Got Time For That
	if len(key) <= 0 || len(val) <= 0 {
		return nil
	}
	var expSeconds int64
	if exp != 0 {
		expSeconds = time.Now().Add(exp).Unix()
	}
	return s.stx.Execute(s.sqlInsert, key, val, expSeconds)
}

// Delete entry by key
func (s *Storage) Delete(key string) error {
	// Ain't Nobody Got Time For That
	if len(key) <= 0 {
		return nil
	}
	return s.stx.Execute(s.sqlDelete, key)
}

// Reset all entries, including unexpired
func (s *Storage) Reset() error {
	return s.stx.Execute(s.sqlReset)
}

// Close the database
func (s *Storage) Close() error {
	return s.stx.Commit()
}

// gcTicker starts the gc ticker
// func (s *Storage) gcTicker() {
// 	ticker := time.NewTicker(s.gcInterval)
// 	defer ticker.Stop()
// 	for {
// 		select {
// 		case <-s.done:
// 			return
// 		case t := <-ticker.C:
// 			s.gc(t)
// 		}
// 	}
// }

// gc deletes all expired entries
// func (s *Storage) gc(t time.Time) {
// 	_, _ = s.stx.Exec(s.sqlGC, t.Unix())
// }

// func (s *Storage) checkSchema(tableName string) {
// 	var data []byte

// 	row := s.stx.QueryRow(fmt.Sprintf(checkSchemaQuery, tableName))
// 	if err := row.Scan(&data); err != nil {
// 		panic(err)
// 	}

// 	if strings.ToLower(string(data)) != "bytea" {
// 		fmt.Printf(checkSchemaMsg, string(data))
// 	}
// }
