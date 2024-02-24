// Package: main
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// DBConfig holds the database configuration details
type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

// UserDetail holds the details of a user including their roles
type UserDetail struct {
	ID       int
	Username string
	Name     string
	Email    string
	Roles    []string
}

// connectDB initializes a connection to the database using the provided DBConfig
func connectDB(config DBConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4,utf8&parseTime=True",
		config.User, config.Password, config.Host, config.Port, config.DBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}
	return db, nil
}

func main() {

	filePath := "wp-config.php"
	_, err := os.Stat(filePath)

	if err == nil {
		dbName, dbUser, dbPassword, err := extractWPDBDetails(filePath)
		if err != nil {
			fmt.Println("Error reading file:", err)
		}

		fmt.Println("DB Name:", dbName)
		fmt.Println("DB User:", dbUser)

		// Example usage
		config := DBConfig{
			Host:     "localhost",
			Port:     3306,
			User:     dbUser,
			Password: dbPassword,
			DBName:   dbName,
		}

		prefixesWP, err := identifyWPPrefixes(config)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Identified WordPress table prefixes:", strings.Join(prefixesWP, ", "))

		for _, prefix := range prefixesWP {
			listWordPressUsers(config, prefix)
		}
	}

	filePath = "configuration.php"
	_, err = os.Stat(filePath)

	if err == nil {
		dbName, dbUser, dbPassword, _, err := extractJoomlaDBConfig(filePath)
		if err != nil {
			fmt.Println("Error reading file:", err)
		}

		// Example usage
		config := DBConfig{
			Host:     "localhost",
			Port:     3306,
			User:     dbUser,
			Password: dbPassword,
			DBName:   dbName,
		}

		fmt.Println("DB Name:", dbName)
		fmt.Println("DB User:", dbUser)
		fmt.Println()

		db, err := connectDB(config)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()

		prefixesJoomla, err := identifyJoomlaPrefixes(config)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Identified Joomla table prefixes:", strings.Join(prefixesJoomla, ", "))
		fmt.Println()

		users, err := listJoomlaUsersAcrossPrefixes(db, config)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Identified Joomla users:")
		for _, user := range users {
			// Assuming you want to print the ID, Name, Email, and Roles for each user
			fmt.Printf("ID: %d, Username: %s, Roles: %s, Name: %s, Email: %s\n", user.ID, user.Username, strings.Join(user.Roles, ", "), user.Name, user.Email)
		}
	}
}
