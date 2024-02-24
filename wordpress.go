package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// identifyWPPrefixes finds distinct WordPress table prefixes in the database
func identifyWPPrefixes(config DBConfig) ([]string, error) {
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
		// Looking for known WordPress table patterns to confirm a prefix
		if strings.Contains(table, "_users") || strings.Contains(table, "_posts") {
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

// listWordPressUsers lists all users from the WordPress database using the given prefix and attempts to print out the user type
func listWordPressUsers(config DBConfig, wpPrefix string) {
	db, err := connectDB(config)
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}
	defer db.Close()

	// Explicitly include the underscore in the SQL query string
	query := fmt.Sprintf(`
SELECT u.ID, u.user_login, u.user_email,
MAX(CASE WHEN m.meta_key = '%[1]s_capabilities' THEN m.meta_value ELSE NULL END) AS capabilities,
MAX(CASE WHEN m.meta_key = '%[1]s_first_name' THEN m.meta_value ELSE NULL END) AS first_name,
MAX(CASE WHEN m.meta_key = '%[1]s_last_name' THEN m.meta_value ELSE NULL END) AS last_name,
MAX(CASE WHEN m.meta_key = '%[1]s_nickname' THEN m.meta_value ELSE NULL END) AS nickname
FROM %[1]s_users u
LEFT JOIN %[1]s_usermeta m ON u.ID = m.user_id
GROUP BY u.ID, u.user_login, u.user_email`, wpPrefix)

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Failed to execute query for prefix '%s': %v\n", wpPrefix, err)
		return
	}
	defer rows.Close()

	fmt.Printf("WordPress Users for prefix '%s':\n", wpPrefix)
	for rows.Next() {
		var id int
		var userLogin, userEmail, capabilities string
		var firstName, lastName, nickname sql.NullString // Use sql.NullString for nullable fields

		err = rows.Scan(&id, &userLogin, &userEmail, &capabilities, &firstName, &lastName, &nickname)
		if err != nil {
			log.Fatal("Failed to scan row: ", err)
		}

		role := identifyWPUserRole(capabilities)

		// Safely get the string value, defaulting to an empty string if NULL
		fn := ""
		if firstName.Valid {
			fn = firstName.String
		}

		ln := ""
		if lastName.Valid {
			ln = lastName.String
		}

		nn := ""
		if nickname.Valid {
			nn = nickname.String
		}

		fmt.Printf("ID: %d, Username: %s, Email: %s, Role: %s, First Name: %s, Last Name: %s, Nickname: %s\n",
			id, userLogin, userEmail, role, fn, ln, nn)
	}

	err = rows.Err()
	if err != nil {
		log.Fatal("Error fetching rows: ", err)
	}
}

// identifyWPUserRole attempts to identify the user role from serialized PHP string in wp_capabilities
func identifyWPUserRole(capabilities string) string {
	lowerCaps := strings.ToLower(capabilities) // Lowercase the capabilities to ensure case-insensitive matching.
	if strings.Contains(lowerCaps, "administrator") {
		return "Administrator"
	} else if strings.Contains(lowerCaps, "editor") {
		return "Editor"
	} else if strings.Contains(lowerCaps, "author") {
		return "Author"
	} else if strings.Contains(lowerCaps, "contributor") {
		return "Contributor"
	} else if strings.Contains(lowerCaps, "subscriber") {
		return "Subscriber"
	}
	return "Unknown"
}

// Extracts WP DB details from wp-config.php file.
func extractWPDBDetails(filePath string) (dbName, dbUser, dbPassword string, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", "", "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	dbNamePattern := regexp.MustCompile(`define\(\s*'DB_NAME'\s*,\s*'([^']*)'\s*\);`)
	dbUserPattern := regexp.MustCompile(`define\(\s*'DB_USER'\s*,\s*'([^']*)'\s*\);`)
	dbPasswordPattern := regexp.MustCompile(`define\(\s*'DB_PASSWORD'\s*,\s*'([^']*)'\s*\);`)

	for scanner.Scan() {
		line := scanner.Text()
		if dbNameMatch := dbNamePattern.FindStringSubmatch(line); dbNameMatch != nil {
			dbName = dbNameMatch[1]
		}
		if dbUserMatch := dbUserPattern.FindStringSubmatch(line); dbUserMatch != nil {
			dbUser = dbUserMatch[1]
		}
		if dbPasswordMatch := dbPasswordPattern.FindStringSubmatch(line); dbPasswordMatch != nil {
			dbPassword = dbPasswordMatch[1]
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", "", err
	}

	return dbName, dbUser, dbPassword, nil
}

// changeUserPassword updates the specified user's password with a new one, hashing it as required by WordPress.
func changeWPUserPassword(db *sql.DB, wpPrefix, username, newPassword string) error {
	// Hash the new password using bcrypt, which is compatible with WordPress since version 5.7.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	// Prepare the SQL statement to update the user's password.
	query := fmt.Sprintf("UPDATE %susers SET user_pass = ? WHERE user_login = ?", wpPrefix)
	_, err = db.Exec(query, string(hashedPassword), username)
	if err != nil {
		return fmt.Errorf("failed to update user password: %v", err)
	}

	log.Printf("Password for user '%s' has been updated successfully.", username)
	return nil
}
