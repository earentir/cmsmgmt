package database

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// DBConfig holds the configuration for connecting to a database.
type DBConfig struct {
	Type     string // "mysql" or "postgres"
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

// Connect establishes a connection to the database using the provided configuration.
func Connect(config DBConfig) (*sql.DB, error) {
	var dsn string
	var driverName string

	switch config.Type {
	case "mysql", "mysqli":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True",
			config.User, config.Password, config.Host, config.Port, config.DBName)
		driverName = "mysql"
	case "postgres":
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			config.Host, config.Port, config.User, config.Password, config.DBName)
		driverName = "postgres"
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

// IdentifyPrefixes identifies the prefixes used in the database tables for WordPress and Joomla.
func IdentifyPrefixes(db *sql.DB, dbType string) ([]string, error) {
	var query string
	switch strings.ToLower(dbType) {
	case "mysql", "mysqli":
		query = "SHOW TABLES"
	case "postgres":
		query = `
            SELECT tablename
            FROM   pg_catalog.pg_tables
            WHERE  schemaname NOT IN ('pg_catalog', 'information_schema')`
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %v", err)
	}
	defer rows.Close()

	// track which companion tables we have seen for each prefix
	type flags struct {
		users, posts, userMap, userGroups bool
	}
	seen := make(map[string]*flags)

	for rows.Next() {
		var tbl string
		if err := rows.Scan(&tbl); err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		switch {
		case strings.HasSuffix(tbl, "_users"):
			p := strings.TrimSuffix(tbl, "_users")
			f := seen[p]
			if f == nil {
				f = &flags{}
				seen[p] = f
			}
			f.users = true

		case strings.HasSuffix(tbl, "_posts"):
			p := strings.TrimSuffix(tbl, "_posts")
			f := seen[p]
			if f == nil {
				f = &flags{}
				seen[p] = f
			}
			f.posts = true

		case strings.HasSuffix(tbl, "_user_usergroup_map"):
			p := strings.TrimSuffix(tbl, "_user_usergroup_map")
			f := seen[p]
			if f == nil {
				f = &flags{}
				seen[p] = f
			}
			f.userMap = true

		case strings.HasSuffix(tbl, "_usergroups"):
			p := strings.TrimSuffix(tbl, "_usergroups")
			f := seen[p]
			if f == nil {
				f = &flags{}
				seen[p] = f
			}
			f.userGroups = true
		}
	}

	var prefixes []string
	for p, f := range seen {
		if !f.users {
			continue // never keep a prefix without _users
		}

		// WordPress – users + posts
		// Joomla    – users + (userMap or userGroups)
		if f.posts || (f.userMap && f.userGroups) || (f.userMap || f.userGroups && f.posts) {
			prefixes = append(prefixes, p)
		}
	}

	sort.Strings(prefixes) // deterministic order (optional)
	return prefixes, nil
}
