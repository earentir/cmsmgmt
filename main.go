package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"cmsmgmt/joomla"
	"cmsmgmt/wordpress"

	"github.com/spf13/cobra"
)

var (
	cmsPath    string
	appVersion = "0.1.21"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "cmsum",
		Short:   "Content Management System User Management",
		Long:    "https://github.com/earentir/cmsum",
		Version: appVersion,

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmsPath != "" {
				if _, err := os.Stat(cmsPath); os.IsNotExist(err) {
					return fmt.Errorf("The specified CMS path does not exist: %s", cmsPath)
				}
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVarP(&cmsPath, "path", "p", "", "Path to the CMS root directory")

	usersCmd := &cobra.Command{
		Use:   "users",
		Short: "User management commands",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		Run: func(cmd *cobra.Command, args []string) {
			cmsType := detectCMS()
			if cmsType == "" {
				log.Fatal("Unable to detect CMS type. Make sure you're in the correct directory or specify the correct path using the -p flag.")
			}

			var err error
			if cmsType == "wordpress" {
				err = wordpress.ProcessWordPress(cmsPath)
			} else if cmsType == "joomla" {
				db, cfg, defaultPrefix, err2 := joomla.ProcessJoomla(cmsPath)
				if err2 == nil {
					fmt.Printf("Joomla DB Name: %s\n", cfg.DBName)
					fmt.Printf("Joomla DB User: %s\n", cfg.User)
					fmt.Printf("Identified Joomla table prefixes: %v\n", defaultPrefix)

					users, err3 := joomla.ListUsers(db, defaultPrefix)
					if err3 != nil {
						fmt.Println(fmt.Errorf("list users for prefix %s: %w", defaultPrefix, err3))
					}
					fmt.Printf("\nUsers for prefix '%s':\n", defaultPrefix)
					for _, u := range users {
						fmt.Printf("ID:%d  Username:%s  Name:%s  Email:%s  Roles:%v\n", u.ID, u.Username, u.Name, u.Email, u.Roles)
					}
				}
				err = err2
			}

			if err != nil {
				log.Printf("Error processing %s: %v", cmsType, err)
			}
		},
	}

	userInfoCmd := &cobra.Command{
		Use:   "info",
		Short: "Show user info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("User info functionality not implemented yet.")
		},
	}

	editCmd := &cobra.Command{
		Use:   "edit [USERNAME]",
		Short: "Edit user details",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			username := args[0]
			cmsType := detectCMS()
			if cmsType == "" {
				log.Fatal("Unable to detect CMS type. Make sure you're in the correct directory or specify the correct path using the -p flag.")
			}

			var err error
			if cmsType == "wordpress" {
				err = wordpress.EditUser(cmsPath, username)
			} else if cmsType == "joomla" {
				db, _, defaultPrefix, err2 := joomla.ProcessJoomla(cmsPath)
				if err2 == nil {
					err = joomla.EditUser(db, defaultPrefix, cmsPath, username)
				} else {
					err = err2
				}
			}

			if err != nil {
				log.Printf("Error editing %s user: %v", cmsType, err)
			}
		},
	}

	usersCmd.AddCommand(listCmd)
	usersCmd.AddCommand(userInfoCmd)
	usersCmd.AddCommand(editCmd)

	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Show CMS information",
	}

	generalCmd := &cobra.Command{
		Use:   "general",
		Short: "Show general CMS information",
		Run: func(cmd *cobra.Command, args []string) {
			cmsType := detectCMS()
			if cmsType == "" {
				log.Fatal("Unable to detect CMS type. Make sure you're in the correct directory or specify the correct path using the -p flag.")
			}

			var err error
			if cmsType == "wordpress" {
				err = wordpress.ShowInfo(cmsPath)
			} else if cmsType == "joomla" {
				err = joomla.ShowInfo(cmsPath)
			}

			if err != nil {
				log.Printf("Error showing %s info: %v", cmsType, err)
			}
		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show CMS version information",
		Run: func(cmd *cobra.Command, args []string) {
			cmsType := detectCMS()
			if cmsType == "" {
				log.Fatal("Unable to detect CMS type. Make sure you're in the correct directory or specify the correct path using the -p flag.")
			}

			var version, rel string
			var err error
			if cmsType == "wordpress" {
				version, err = wordpress.GetVersion(cmsPath)
			} else if cmsType == "joomla" {
				version, rel, err = joomla.GetVersion(cmsPath)
			}

			if err != nil {
				log.Printf("Error showing %s version: %v", cmsType, err)
			} else {
				fmt.Printf("%s Version: %s\n", cmsType, version)
				if cmsType == "joomla" {
					fmt.Printf("Release: %s\n", rel)
				}
			}
		},
	}

	infoCmd.AddCommand(generalCmd)
	infoCmd.AddCommand(versionCmd)

	rootCmd.AddCommand(usersCmd)
	rootCmd.AddCommand(infoCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func detectCMS() string {
	wpConfig := filepath.Join(cmsPath, "wp-config.php")
	joomlaConfig := filepath.Join(cmsPath, "configuration.php")

	if _, err := os.Stat(wpConfig); err == nil {
		return "wordpress"
	}
	if _, err := os.Stat(joomlaConfig); err == nil {
		return "joomla"
	}
	return ""
}
