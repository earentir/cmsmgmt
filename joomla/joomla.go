// Package joomla provides functionality to interact with Joomla databases.
package joomla

import (
	"bufio"
	"cmsum/database"
	"database/sql"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// UserDetail represents a Joomla user.
type UserDetail struct {
	ID       int
	Username string
	Name     string
	Email    string
	Roles    []string
}

// ExtractDBConfig extracts the database configuration from the given Joomla configuration file.
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
		"DBType":     regexp.MustCompile(`public \$dbtype\s*=\s*'(.+)';`),
		"DBName":     regexp.MustCompile(`public \$db\s*=\s*'(.+)';`),
		"DBUser":     regexp.MustCompile(`public \$user\s*=\s*'(.+)';`),
		"DBPassword": regexp.MustCompile(`public \$password\s*=\s*'(.+)';`),
		"DBHost":     regexp.MustCompile(`public \$host\s*=\s*'(.+)';`),
	}

	for key, pattern := range patterns {
		matches := pattern.FindStringSubmatch(string(content))
		if len(matches) > 1 {
			switch key {
			case "DBType":
				config.Type = matches[1]
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

// IdentifyPrefixes retrieves the table prefixes from the Joomla database.
func IdentifyPrefixes(db *sql.DB, dbType string) ([]string, error) {
	return database.IdentifyPrefixes(db, dbType)
}

// ListUsersAcrossPrefixes retrieves user details from multiple Joomla databases.
func ListUsersAcrossPrefixes(db *sql.DB, prefixes []string) ([]UserDetail, error) {
	var allUsers []UserDetail
	for _, prefix := range prefixes {
		query := fmt.Sprintf(`SELECT u.id, u.username, u.name, u.email, ug.title AS role
							  FROM %s_users AS u
							  INNER JOIN %s_user_usergroup_map AS map ON u.id = map.user_id
							  INNER JOIN %s_usergroups AS ug ON map.group_id = ug.id`, prefix, prefix, prefix)

		rows, err := db.Query(query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var id int
			var name, email, role, username string

			if err := rows.Scan(&id, &username, &name, &email, &role); err != nil {
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
	}

	return allUsers, nil
}

// GetVersion returns the full Joomla version, e.g. "3.10.6 (Stable)" or "4.4.2 (Stable)".
func GetVersion(cmsPath string) (string, error) {
	versionFile := filepath.Join(cmsPath, "libraries", "src", "Version.php")
	contentBytes, err := os.ReadFile(versionFile)
	if err != nil {
		return "", fmt.Errorf("failed to read Joomla version file: %w", err)
	}
	content := string(contentBytes)

	// helper to cut repetition
	get := func(re *regexp.Regexp) string {
		if m := re.FindStringSubmatch(content); len(m) == 2 {
			return m[1]
		}
		return ""
	}

	reRelease := regexp.MustCompile(`(?m)const\s+RELEASE\s*=\s*'([^']+)';`)
	rePatch := regexp.MustCompile(`(?m)const\s+DEV_LEVEL\s*=\s*'([^']+)';`)
	// Accept either DEV_STATUS (J 3) or RELTYPE (J 4)
	reStatus := regexp.MustCompile(`(?m)const\s+(?:DEV_STATUS|RELTYPE)\s*=\s*'([^']+)';`)

	release := get(reRelease)
	if release == "" {
		return "", fmt.Errorf("RELEASE not found in Version.php")
	}

	patch := get(rePatch)   // empty if the constant is missing
	status := get(reStatus) // empty if neither constant present

	version := release
	if patch != "" {
		version += "." + patch
	}
	if status != "" {
		version += " (" + status + ")"
	}
	return version, nil
}

// GetUserByUsername retrieves a user by their username.
func GetUserByUsername(db *sql.DB, username string) (UserDetail, error) {
	query := `
		SELECT u.id, u.username, u.name, u.email, GROUP_CONCAT(ug.title) AS roles
		FROM jos_users u
		LEFT JOIN jos_user_usergroup_map m ON u.id = m.user_id
		LEFT JOIN jos_usergroups ug ON m.group_id = ug.id
		WHERE u.username = ?
		GROUP BY u.id, u.username, u.name, u.email`

	var user UserDetail
	var roles sql.NullString
	err := db.QueryRow(query, username).Scan(&user.ID, &user.Username, &user.Name, &user.Email, &roles)
	if err != nil {
		return UserDetail{}, fmt.Errorf("failed to get user: %v", err)
	}

	if roles.Valid {
		user.Roles = strings.Split(roles.String, ",")
	}

	return user, nil
}

// UpdateUser updates the user details in the Joomla database.
func UpdateUser(db *sql.DB, user UserDetail) error {
	_, err := db.Exec("UPDATE jos_users SET name = ?, email = ? WHERE id = ?",
		user.Name, user.Email, user.ID)
	if err != nil {
		return fmt.Errorf("failed to update user: %v", err)
	}

	return nil
}

func ProcessJoomla(cmsPath string) error {
	configPath := filepath.Join(cmsPath, "configuration.php")
	config, err := ExtractDBConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to extract Joomla DB config: %v", err)
	}

	db, err := database.Connect(config)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}
	defer db.Close()

	prefixes, err := IdentifyPrefixes(db, config.Type)
	if err != nil {
		return fmt.Errorf("failed to identify Joomla prefixes: %v", err)
	}

	fmt.Printf("Joomla DB Name: %s\n", config.DBName)
	fmt.Printf("Joomla DB User: %s\n", config.User)
	fmt.Printf("Identified Joomla table prefixes: %v\n", prefixes)

	users, err := ListUsersAcrossPrefixes(db, prefixes)
	if err != nil {
		return fmt.Errorf("failed to list Joomla users: %v", err)
	}

	fmt.Println("Joomla Users:")
	for _, user := range users {
		fmt.Printf("ID: %d, Username: %s, Name: %s, Email: %s, Roles: %v\n",
			user.ID, user.Username, user.Name, user.Email, user.Roles)
	}

	return nil
}

func ShowInfo(cmsPath string) error {
	configPath := filepath.Join(cmsPath, "configuration.php")
	config, err := ExtractDBConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to extract Joomla DB config: %v", err)
	}

	db, err := database.Connect(config)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}
	defer db.Close()

	prefixes, err := IdentifyPrefixes(db, config.Type)
	if err != nil {
		return fmt.Errorf("failed to identify Joomla prefixes: %v", err)
	}

	fmt.Println("Joomla Information:")
	fmt.Printf("DB Type: %s\n", config.Type)
	fmt.Printf("DB Name: %s\n", config.DBName)
	fmt.Printf("DB User: %s\n", config.User)
	fmt.Printf("DB Host: %s\n", config.Host)
	fmt.Printf("DB Port: %d\n", config.Port)
	fmt.Printf("Table Prefixes: %v\n", prefixes)

	return nil
}

func EditUser(cmsPath, username string) error {
	configPath := filepath.Join(cmsPath, "configuration.php")
	config, err := ExtractDBConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to extract Joomla DB config: %v", err)
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
	fmt.Printf("ID: %d\n", user.ID)
	fmt.Printf("Username: %s\n", user.Username)
	fmt.Printf("Name: %s\n", user.Name)
	fmt.Printf("Email: %s\n", user.Email)
	fmt.Printf("Roles: %v\n", user.Roles)

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter new Name (or press Enter to keep current value): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		user.Name = input
	}

	fmt.Print("Enter new Email (or press Enter to keep current value): ")
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		user.Email = input
	}

	if err := UpdateUser(db, user); err != nil {
		return fmt.Errorf("failed to update user: %v", err)
	}

	fmt.Println("User updated successfully")
	return nil
}
