package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// identifyJoomlaPrefixes finds distinct Joomla table prefixes in the database
func identifyJoomlaPrefixes(config DBConfig) ([]string, error) {
	db, err := connectDB(config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}
	defer db.Close()

	query := "SHOW TABLES"
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
		// Looking for known Joomla table patterns to confirm a prefix
		if strings.Contains(table, "_users") {
			// Extract prefix
			matches := regexp.MustCompile(`^(.+?)_`).FindStringSubmatch(table)
			if len(matches) > 1 {
				prefixSet[matches[1]] = true
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error fetching rows: %v", err)
	}

	var prefixes []string
	for prefix := range prefixSet {
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

// listJoomlaUsersAcrossPrefixes lists all users from all Joomla user tables identified by their prefixes
func listJoomlaUsersAcrossPrefixes(db *sql.DB, config DBConfig) ([]UserDetail, error) {
	prefixes, err := identifyJoomlaPrefixes(config)
	if err != nil {
		return nil, err
	}

	var allUsers []UserDetail
	for _, prefix := range prefixes {
		usersQuery := fmt.Sprintf(`SELECT u.id, u.username, u.name, u.email, ug.title AS role
									FROM %s_users AS u
									INNER JOIN %s_user_usergroup_map AS map ON u.id = map.user_id
									INNER JOIN %s_usergroups AS ug ON map.group_id = ug.id`, prefix, prefix, prefix)

		rows, err := db.Query(usersQuery)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var id int
			var name, email, role, username string

			if err := rows.Scan(&id, &username, &name, &email, &role); err != nil {
				rows.Close()
				return nil, err
			}

			userDetail := UserDetail{
				ID:       id,
				Username: username,
				Name:     name,
				Email:    email,
				Roles:    []string{role},
			}

			allUsers = append(allUsers, userDetail)
		}
		rows.Close() // Ensure rows are closed after processing each prefix
	}

	return allUsers, nil
}

// Extracts Joomla database configuration details from configuration.php.
func extractJoomlaDBConfig(filePath string) (dbName, dbUser, dbPassword, dbPrefix string, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", "", "", "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		// Use strings.Contains to check if line contains the variable
		if strings.Contains(line, "$db =") {
			dbName = extractValue(line)
		} else if strings.Contains(line, "$user =") {
			dbUser = extractValue(line)
		} else if strings.Contains(line, "$password =") {
			dbPassword = extractValue(line)
		} else if strings.Contains(line, "$dbprefix =") {
			dbPrefix = extractValue(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", "", "", err
	}

	return dbName, dbUser, dbPassword, dbPrefix, nil
}

// extractValue takes a line containing a Joomla configuration variable assignment
// and extracts the value assigned to the variable.
func extractValue(line string) string {
	start := strings.Index(line, "'") + 1 // Find the index of the first single quote
	end := strings.LastIndex(line, "'")   // Find the index of the last single quote
	if start < end {
		return line[start:end]
	}
	return ""
}
