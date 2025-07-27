package joomla

import (
	"bufio"
	"cmsmgmt/database"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
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
// It also returns the configured table prefix, if found, to speed up later look‑ups.
func ExtractDBConfig(filePath string) (database.DBConfig, string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return database.DBConfig{}, "", err
	}

	cfg := database.DBConfig{
		Type: "mysql", // default to MySQL
		Port: 3306,    // default MySQL port
	}
	var dbPrefix string

	patterns := map[string]*regexp.Regexp{
		"DBType":     regexp.MustCompile(`public \$dbtype\s*=\s*'([^']+)';`),
		"DBName":     regexp.MustCompile(`public \$db\s*=\s*'([^']+)';`),
		"DBUser":     regexp.MustCompile(`public \$user\s*=\s*'([^']+)';`),
		"DBPassword": regexp.MustCompile(`public \$password\s*=\s*'([^']+)';`),
		"DBHost":     regexp.MustCompile(`public \$host\s*=\s*'([^']+)';`),
		"DBPrefix":   regexp.MustCompile(`public \$dbprefix\s*=\s*'([^']+)';`),
	}

	for key, re := range patterns {
		if m := re.FindStringSubmatch(string(content)); len(m) > 1 {
			switch key {
			case "DBType":
				t := strings.ToLower(m[1])
				if t == "mysqli" {
					t = "mysql"
				}
				cfg.Type = t
			case "DBName":
				cfg.DBName = m[1]
			case "DBUser":
				cfg.User = m[1]
			case "DBPassword":
				cfg.Password = m[1]
			case "DBHost":
				hostPort := m[1]
				if h, p, err := net.SplitHostPort(hostPort); err == nil {
					cfg.Host = h
					if pn, err := strconv.Atoi(p); err == nil {
						cfg.Port = pn
					}
				} else {
					cfg.Host = hostPort
				}
			case "DBPrefix":
				// trim trailing underscore if present
				dbPrefix = strings.TrimSuffix(m[1], "_")
			}
		}
	}
	return cfg, dbPrefix, nil
}

// IdentifyPrefixes returns prefixes that really belong to Joomla installations.
func IdentifyPrefixes(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SHOW TABLES LIKE '%\\_users'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prefixes []string
	for rows.Next() {
		var tbl string
		if err := rows.Scan(&tbl); err != nil {
			return nil, err
		}
		prefix := strings.TrimSuffix(tbl, "_users")
		// check companion tables exist
		need := []string{prefix + "_user_usergroup_map", prefix + "_usergroups"}
		ok := true
		for _, t := range need {
			var dummy string
			if err := db.QueryRow("SHOW TABLES LIKE ?", t).Scan(&dummy); err != nil {
				ok = false
				break
			}
		}
		if ok {
			prefixes = append(prefixes, prefix)
		}
	}
	sort.Strings(prefixes)
	return prefixes, nil
}

// ListUsers retrieves user details for a single prefix.
func ListUsers(db *sql.DB, prefix string) ([]UserDetail, error) {
	q := fmt.Sprintf(`
        SELECT u.id, u.username, u.name, u.email,
               GROUP_CONCAT(ug.title SEPARATOR ',') AS roles
        FROM %s_users u
        LEFT JOIN %s_user_usergroup_map m ON u.id = m.user_id
        LEFT JOIN %s_usergroups ug ON m.group_id = ug.id
        GROUP BY u.id`, prefix, prefix, prefix)
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserDetail
	for rows.Next() {
		var u UserDetail
		var roles sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.Name, &u.Email, &roles); err != nil {
			return nil, err
		}
		if roles.Valid {
			u.Roles = strings.Split(roles.String, ",")
		}
		users = append(users, u)
	}
	return users, nil
}

// GetUserByUsername retrieves a user by username for the given prefix.
func GetUserByUsername(db *sql.DB, prefix, username string) (UserDetail, error) {
	q := fmt.Sprintf(`SELECT u.id, u.username, u.name, u.email,
                             GROUP_CONCAT(ug.title) AS roles
                      FROM %[1]s_users u
                      LEFT JOIN %[1]s_user_usergroup_map m ON u.id = m.user_id
                      LEFT JOIN %[1]s_usergroups ug        ON m.group_id = ug.id
                      WHERE u.username = ?
                      GROUP BY u.id`, prefix)
	var u UserDetail
	var roles sql.NullString
	if err := db.QueryRow(q, username).Scan(&u.ID, &u.Username, &u.Name, &u.Email, &roles); err != nil {
		return UserDetail{}, err
	}
	if roles.Valid {
		u.Roles = strings.Split(roles.String, ",")
	}
	return u, nil
}

// UpdateUser updates name & e‑mail in the relevant tables for a given prefix.
func UpdateUser(db *sql.DB, prefix string, u UserDetail) error {
	_, err := db.Exec(fmt.Sprintf("UPDATE %s_users SET name = ?, email = ? WHERE id = ?", prefix), u.Name, u.Email, u.ID)
	return err
}

// ---------------- public entry points ----------------

// ProcessJoomla processes the Joomla installation at the given path.
func ProcessJoomla(cmsPath string) (db *sql.DB, cfg database.DBConfig, defaultPrefix string, err error) {
	// 1) Read Joomla config
	configPath := filepath.Join(cmsPath, "configuration.php")
	cfg, defaultPrefix, err = ExtractDBConfig(configPath)
	if err != nil {
		return nil, cfg, "", fmt.Errorf("failed to extract Joomla DB config: %w", err)
	}

	// 2) Connect to DB
	db, err = database.Connect(cfg)
	if err != nil {
		return nil, cfg, "", fmt.Errorf("failed to connect to database: %w", err)
	}

	// 3) Identify table prefixes
	prefixes, err := IdentifyPrefixes(db)
	if err != nil {
		db.Close()
		return nil, cfg, "", fmt.Errorf("failed to identify Joomla prefixes: %w", err)
	}
	if len(prefixes) == 0 && defaultPrefix != "" {
		prefixes = []string{defaultPrefix}
	}

	// return db (open) and prefixes
	return db, cfg, defaultPrefix, nil
}

// ShowInfo displays general information about the Joomla installation.
func ShowInfo(cmsPath string) error {
	cfgPath := filepath.Join(cmsPath, "configuration.php")
	cfg, _, err := ExtractDBConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("extract Joomla DB config: %w", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer db.Close()

	prefixes, _ := IdentifyPrefixes(db)

	fmt.Println("Joomla Information:")
	fmt.Printf("DB Type  : %s\n", cfg.Type)
	fmt.Printf("DB Name  : %s\n", cfg.DBName)
	fmt.Printf("DB User  : %s\n", cfg.User)
	fmt.Printf("DB Host  : %s\n", cfg.Host)
	fmt.Printf("DB Port  : %d\n", cfg.Port)
	fmt.Printf("Prefixes : %v\n", prefixes)
	return nil
}

// EditUser allows editing user details in the Joomla database.
func EditUser(db *sql.DB, prefix, cmsPath, username string) error {
	// 1) load
	user, err := GetUserByUsername(db, prefix, username)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	reader := bufio.NewReader(os.Stdin)

	// 2) read inputs...
	fmt.Print("New Name (Enter to keep): ")
	nameIn, _ := reader.ReadString('\n')
	name := strings.TrimSpace(nameIn)
	if name == "" {
		name = user.Name
	}

	fmt.Print("New Email (Enter to keep): ")
	emailIn, _ := reader.ReadString('\n')
	email := strings.TrimSpace(emailIn)
	if email == "" {
		email = user.Email
	}

	fmt.Print("New Password (Enter to keep): ")
	passIn, _ := reader.ReadString('\n')
	pass := strings.TrimSpace(passIn)

	fmt.Printf("Current Roles: %v\n", user.Roles)
	fmt.Print("New Roles CSV (Enter to keep): ")
	rolesIn, _ := reader.ReadString('\n')
	rolesCSV := strings.TrimSpace(rolesIn)

	// 3) begin transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	// 4) password update
	if pass != "" {
		hashed, err := joomlaHashAuto(cmsPath, pass)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("hash password: %w", err)
		}
		fmt.Println("Hashed password:", hashed)

		res, err := tx.Exec(
			fmt.Sprintf("UPDATE `%s_users` SET password = ? WHERE id = ?", prefix),
			hashed, user.ID,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("update password: %w", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			tx.Rollback()
			return fmt.Errorf("password update affected %d rows", n)
		}
	}

	// 5) roles update
	if rolesCSV != "" {
		if _, err := tx.Exec(
			fmt.Sprintf("DELETE FROM `%s_user_usergroup_map` WHERE user_id = ?", prefix),
			user.ID,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("clear roles: %w", err)
		}
		for _, r := range strings.Split(rolesCSV, ",") {
			title := strings.TrimSpace(r)
			var gid int
			if err := tx.QueryRow(
				fmt.Sprintf("SELECT id FROM `%s_usergroups` WHERE title = ?", prefix),
				title,
			).Scan(&gid); err == nil {
				if _, err := tx.Exec(
					fmt.Sprintf("INSERT INTO `%s_user_usergroup_map` (user_id, group_id) VALUES (?,?)", prefix),
					user.ID, gid,
				); err != nil {
					tx.Rollback()
					return fmt.Errorf("insert role %q: %w", title, err)
				}
			}
		}
	}

	// 6) name/email update
	if name != user.Name || email != user.Email {
		res, err := tx.Exec(
			fmt.Sprintf("UPDATE `%s_users` SET name = ?, email = ? WHERE id = ?", prefix),
			name, email, user.ID,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("update name/email: %w", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			tx.Rollback()
			return fmt.Errorf("name/email update affected %d rows", n)
		}
	}

	// 7) commit
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	fmt.Println("User updated successfully.")
	return nil
}

// GetVersion returns the full Joomla version, e.g. "3.10.6 (Stable)" or "4.4.2 (Stable)".
func GetVersion(cmsPath string) (version string, relDate string, err error) {
	// 1) Try the "old" property‑style file (Joomla 2.5 → 3.x < 3.8)
	oldPath := filepath.Join(cmsPath, "libraries", "cms", "version", "version.php")
	if buf, readErr := os.ReadFile(oldPath); readErr == nil {
		content := string(buf)

		// property‑style regexes
		reRel := regexp.MustCompile(`(?m)public\s+\$RELEASE\s*=\s*'([^']+)';`)
		reLev := regexp.MustCompile(`(?m)public\s+\$DEV_LEVEL\s*=\s*'([^']+)';`)
		reStat := regexp.MustCompile(`(?m)public\s+\$DEV_STATUS\s*=\s*'([^']+)';`)
		reRelDat := regexp.MustCompile(`(?m)public\s+\$RELDATE\s*=\s*'([^']+)';`)

		get := func(r *regexp.Regexp) string {
			if m := r.FindStringSubmatch(content); len(m) == 2 {
				return m[1]
			}
			return ""
		}

		rel := get(reRel)
		if rel == "" {
			return "", "", fmt.Errorf("no RELEASE found in %s", oldPath)
		}

		version = rel
		if lvl := get(reLev); lvl != "" {
			version += "." + lvl
		}
		if st := get(reStat); st != "" {
			version += " (" + st + ")"
		}
		relDate = get(reRelDat) // may be empty if not set
		return version, relDate, nil
	}

	// 2) Fall back to the PSR‑4 constant‑style file (Joomla 3.8+)
	newPath := filepath.Join(cmsPath, "libraries", "src", "Version.php")
	buf, err := os.ReadFile(newPath)
	if err != nil {
		return "", "", fmt.Errorf(
			"could not find either Joomla 2.5–3.x file (%s) or PSR‑4 file (%s): %w",
			oldPath, newPath, err,
		)
	}
	content := string(buf)

	// constants for Joomla 3.x
	reCRel := regexp.MustCompile(`(?m)const\s+RELEASE\s*=\s*'([^']+)';`)
	reCPatch := regexp.MustCompile(`(?m)const\s+DEV_LEVEL\s*=\s*'([^']+)';`)
	reCStat := regexp.MustCompile(`(?m)const\s+(?:DEV_STATUS|RELTYPE)\s*=\s*'([^']+)';`)
	// constants for Joomla 4.x
	reMajor := regexp.MustCompile(`(?m)const\s+MAJOR_VERSION\s*=\s*([0-9]+);`)
	reMinor := regexp.MustCompile(`(?m)const\s+MINOR_VERSION\s*=\s*([0-9]+);`)
	reP4Patch := regexp.MustCompile(`(?m)const\s+PATCH_VERSION\s*=\s*([0-9]+);`)
	reExtra := regexp.MustCompile(`(?m)const\s+EXTRA_VERSION\s*=\s*'([^']*)';`)
	// optional release‑date constant (if ever added)
	reCRelDat := regexp.MustCompile(`(?m)const\s+RELDATE\s*=\s*'([^']+)';`)

	getC := func(r *regexp.Regexp) string {
		if m := r.FindStringSubmatch(content); len(m) == 2 {
			return m[1]
		}
		return ""
	}

	// 2a) Try Joomla 3.x style first
	if rel := getC(reCRel); rel != "" {
		version = rel
		if p := getC(reCPatch); p != "" {
			version += "." + p
		}
		if s := getC(reCStat); s != "" {
			version += " (" + s + ")"
		}
		relDate = getC(reCRelDat)
		return version, relDate, nil
	}

	// 2b) Otherwise Joomla 4.x style
	maj := getC(reMajor)
	min := getC(reMinor)
	if maj == "" || min == "" {
		return "", "", fmt.Errorf("could not parse Joomla constants in %s", newPath)
	}
	version = maj + "." + min
	if p := getC(reP4Patch); p != "" && p != "0" {
		version += "." + p
	}
	if e := getC(reExtra); e != "" {
		version += "-" + e
	}
	relDate = getC(reCRelDat)
	return version, relDate, nil
}

// parseMajorVersion turns "3.10.6" or "4.2.0 (Stable)" into 3 or 4
func parseMajorVersion(v string) (int, error) {
	// split on dot or space
	f := strings.FieldsFunc(v, func(r rune) bool {
		return r == '.' || r == ' '
	})
	if len(f) == 0 {
		return 0, fmt.Errorf("invalid version format: %q", v)
	}
	return strconv.Atoi(f[0])
}

// joomlaHashAuto picks the right algorithm based on the installed Joomla version.
func joomlaHashAuto(cmsPath, password string) (string, error) {
	ver, _, err := GetVersion(cmsPath)
	var major int
	if err != nil {
		// Could not read Version.php — assume Joomla 1.5/2.5
		major = 2
	} else {
		major, err = parseMajorVersion(ver)
		if err != nil {
			return "", fmt.Errorf("parse major version %q: %w", ver, err)
		}
	}

	if major < 3 {
		// MD5+salt for legacy
		saltBytes := make([]byte, 16)
		if _, err := rand.Read(saltBytes); err != nil {
			return "", fmt.Errorf("salt gen: %w", err)
		}
		salt := hex.EncodeToString(saltBytes)
		sum := md5.Sum([]byte(password + salt))
		return fmt.Sprintf("%x:%s", sum, salt), nil
	}

	// bcrypt for 3,4,5
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(hash), nil
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
