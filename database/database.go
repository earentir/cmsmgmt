package database

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type DBConfig struct {
	Type     string // "mysql" or "postgres"
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

func Connect(config DBConfig) (*sql.DB, error) {
	var dsn string
	var driverName string

	switch config.Type {
	case "mysql":
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

func IdentifyPrefixes(db *sql.DB, dbType string) ([]string, error) {
	var query string
	switch dbType {
	case "mysql":
		query = "SHOW TABLES"
	case "postgres":
		query = "SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname != 'pg_catalog' AND schemaname != 'information_schema'"
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %v", err)
	}
	defer rows.Close()

	prefixSet := make(map[string]bool)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		// This logic might need to be adjusted based on the specific naming conventions of WordPress and Joomla
		if len(table) > 6 && (table[len(table)-6:] == "_users" || table[len(table)-6:] == "_posts") {
			prefix := table[:len(table)-6]
			prefixSet[prefix] = true
		}
	}

	var prefixes []string
	for prefix := range prefixSet {
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}
