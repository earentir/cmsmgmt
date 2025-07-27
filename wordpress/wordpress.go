// Package wordpress provides functions to interact with WordPress installations.
package wordpress

import (
	"bufio"
	"cmsmgmt/database"
	"database/sql"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ExtractDBConfig extracts the database configuration from the given WordPress configuration file.
func ExtractDBConfig(filePath string) (database.DBConfig, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return database.DBConfig{}, err
	}

	config := database.DBConfig{
		Type: "mysql", // Default to MySQL
		Port: 3306,    // Default MySQL port
	}

	patterns := map[string]*regexp.Regexp{
		"DBName":     regexp.MustCompile(`define\(\s*'DB_NAME',\s*'(.+)'\s*\)`),
		"DBUser":     regexp.MustCompile(`define\(\s*'DB_USER',\s*'(.+)'\s*\)`),
		"DBPassword": regexp.MustCompile(`define\(\s*'DB_PASSWORD',\s*'(.+)'\s*\)`),
		"DBHost":     regexp.MustCompile(`define\(\s*'DB_HOST',\s*'(.+)'\s*\)`),
	}

	for key, pattern := range patterns {
		matches := pattern.FindStringSubmatch(string(content))
		if len(matches) > 1 {
			switch key {
			case "DBName":
				config.DBName = matches[1]
			case "DBUser":
				config.User = matches[1]
			case "DBPassword":
				config.Password = matches[1]
			case "DBHost":
				hostPort := matches[1]
				if host, port, err := net.SplitHostPort(hostPort); err == nil {
					config.Host = host
					if portNum, err := strconv.Atoi(port); err == nil {
						config.Port = portNum
					}
				} else {
					config.Host = hostPort
				}
			}
		}
	}

	return config, nil
}

// IdentifyPrefixes identifies the table prefixes used in the WordPress database.
func IdentifyPrefixes(db *sql.DB, dbType string) ([]string, error) {
	return database.IdentifyPrefixes(db, dbType)
}

// ListUsers retrieves the list of users from the WordPress database with the given table prefix.
func ListUsers(db *sql.DB, prefix string) ([]map[string]string, error) {
	query := fmt.Sprintf(`
		SELECT u.ID, u.user_login, u.user_email, u.display_name,
		   MAX(CASE WHEN m.meta_key = '%[1]s_capabilities' THEN m.meta_value ELSE NULL END) AS capabilities,
		   MAX(CASE WHEN m.meta_key = 'first_name' THEN m.meta_value ELSE NULL END) AS first_name,
		   MAX(CASE WHEN m.meta_key = 'last_name' THEN m.meta_value ELSE NULL END) AS last_name,
		   MAX(CASE WHEN m.meta_key = 'nickname' THEN m.meta_value ELSE NULL END) AS nickname
		FROM %[1]s_users u
		LEFT JOIN %[1]s_usermeta m ON u.ID = m.user_id
		GROUP BY u.ID, u.user_login, u.user_email, u.display_name`, prefix)

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %v", err)
	}
	defer rows.Close()

	var users []map[string]string
	for rows.Next() {
		var id, login, email, displayName string
		var capabilities, firstName, lastName, nickname sql.NullString
		err := rows.Scan(&id, &login, &email, &displayName, &capabilities, &firstName, &lastName, &nickname)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		user := map[string]string{
			"ID":       id,
			"Username": login,
			"Email":    email,
			"Name":     displayName,
			"Role":     identifyUserRole(capabilities.String),
		}

		if firstName.Valid {
			user["FirstName"] = firstName.String
		}
		if lastName.Valid {
			user["LastName"] = lastName.String
		}
		if nickname.Valid {
			user["Nickname"] = nickname.String
		}

		users = append(users, user)
	}

	return users, nil
}

// GetVersion retrieves the version of WordPress from the given path.
func GetVersion(cmsPath string) (string, error) {
	versionFile := filepath.Join(cmsPath, "wp-includes", "version.php")
	content, err := os.ReadFile(versionFile)
	if err != nil {
		return "", fmt.Errorf("failed to read WordPress version file: %v", err)
	}

	re := regexp.MustCompile(`\$wp_version = '(.+)';`)
	matches := re.FindStringSubmatch(string(content))

	if len(matches) < 2 {
		return "", fmt.Errorf("could not find WordPress version in version.php")
	}

	return matches[1], nil
}

// identifyUserRole identifies the role of a user based on the capabilities string.
func identifyUserRole(capabilities string) string {
	lowerCaps := strings.ToLower(capabilities)
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

// GetUserByUsername retrieves the user details from the WordPress database with the given username.
func GetUserByUsername(db *sql.DB, username string) (map[string]string, error) {
	query := `
		SELECT u.ID, u.user_login, u.user_email, u.display_name,
		   MAX(CASE WHEN m.meta_key = 'first_name' THEN m.meta_value ELSE NULL END) AS first_name,
		   MAX(CASE WHEN m.meta_key = 'last_name' THEN m.meta_value ELSE NULL END) AS last_name,
		   MAX(CASE WHEN m.meta_key = 'nickname' THEN m.meta_value ELSE NULL END) AS nickname
		FROM wp_users u
		LEFT JOIN wp_usermeta m ON u.ID = m.user_id
		WHERE u.user_login = ?
		GROUP BY u.ID, u.user_login, u.user_email, u.display_name`

	var id, login, email, displayName string
	var firstName, lastName, nickname sql.NullString
	err := db.QueryRow(query, username).Scan(&id, &login, &email, &displayName, &firstName, &lastName, &nickname)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	user := map[string]string{
		"ID":       id,
		"Username": login,
		"Email":    email,
		"Name":     displayName,
	}

	if firstName.Valid {
		user["FirstName"] = firstName.String
	}
	if lastName.Valid {
		user["LastName"] = lastName.String
	}
	if nickname.Valid {
		user["Nickname"] = nickname.String
	}

	return user, nil
}

// UpdateUser updates the user details in the WordPress database.
func UpdateUser(db *sql.DB, user map[string]string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Update wp_users table
	_, err = tx.Exec("UPDATE wp_users SET user_email = ?, display_name = ? WHERE ID = ?",
		user["Email"], user["Name"], user["ID"])
	if err != nil {
		return fmt.Errorf("failed to update user: %v", err)
	}

	// Update wp_usermeta table
	metaFields := map[string]string{
		"first_name": "FirstName",
		"last_name":  "LastName",
		"nickname":   "Nickname",
	}

	for metaKey, userKey := range metaFields {
		if value, ok := user[userKey]; ok {
			_, err = tx.Exec("UPDATE wp_usermeta SET meta_value = ? WHERE user_id = ? AND meta_key = ?",
				value, user["ID"], metaKey)
			if err != nil {
				return fmt.Errorf("failed to update user meta %s: %v", metaKey, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func ProcessWordPress(cmsPath string) error {
	configPath := filepath.Join(cmsPath, "wp-config.php")
	config, err := ExtractDBConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to extract WordPress DB config: %v", err)
	}

	db, err := database.Connect(config)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}
	defer db.Close()

	prefixes, err := IdentifyPrefixes(db, config.Type)
	if err != nil {
		return fmt.Errorf("failed to identify WordPress prefixes: %v", err)
	}

	fmt.Printf("WordPress DB Name: %s\n", config.DBName)
	fmt.Printf("WordPress DB User: %s\n", config.User)
	fmt.Printf("Identified WordPress table prefixes: %v\n", prefixes)

	for _, prefix := range prefixes {
		users, err := ListUsers(db, prefix)
		if err != nil {
			return fmt.Errorf("failed to list WordPress users for prefix %s: %v", prefix, err)
		}
		fmt.Printf("WordPress Users for prefix '%s':\n", prefix)
		for _, user := range users {
			fmt.Printf("ID: %s, Username: %s, Email: %s, Role: %s, Name: %s %s, Nickname: %s\n",
				user["ID"], user["Username"], user["Email"], user["Role"],
				user["FirstName"], user["LastName"], user["Nickname"])
		}
	}

	return nil
}

func ShowInfo(cmsPath string) error {
	configPath := filepath.Join(cmsPath, "wp-config.php")
	config, err := ExtractDBConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to extract WordPress DB config: %v", err)
	}

	db, err := database.Connect(config)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}
	defer db.Close()

	prefixes, err := IdentifyPrefixes(db, config.Type)
	if err != nil {
		return fmt.Errorf("failed to identify WordPress prefixes: %v", err)
	}

	fmt.Println("WordPress Information:")
	fmt.Printf("DB Type: %s\n", config.Type)
	fmt.Printf("DB Name: %s\n", config.DBName)
	fmt.Printf("DB User: %s\n", config.User)
	fmt.Printf("DB Host: %s\n", config.Host)
	fmt.Printf("DB Port: %d\n", config.Port)
	fmt.Printf("Table Prefixes: %v\n", prefixes)

	return nil
}

func EditUser(cmsPath, username string) error {
	configPath := filepath.Join(cmsPath, "wp-config.php")
	config, err := ExtractDBConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to extract WordPress DB config: %v", err)
	}

	db, err := database.Connect(config)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}
	defer db.Close()

	user, err := GetUserByUsername(db, username)
	if err != nil {
		return fmt.Errorf("failed to get user: %v", err)
	}

	fmt.Println("Current user details:")
	for key, value := range user {
		if key != "ID" && key != "Password" {
			fmt.Printf("%s: %s\n", key, value)
		}
	}

	reader := bufio.NewReader(os.Stdin)
	for key := range user {
		if key != "ID" && key != "Password" {
			fmt.Printf("Enter new %s (or press Enter to keep current value): ", key)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				user[key] = input
			}
		}
	}

	if err := UpdateUser(db, user); err != nil {
		return fmt.Errorf("failed to update user: %v", err)
	}

	fmt.Println("User updated successfully")
	return nil
}
