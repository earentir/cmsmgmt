package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"cmsum/joomla"
	"cmsum/wordpress"

	cli "github.com/jawher/mow.cli"
)

var cmsPath string

func main() {
	app := cli.App("cmsum", "Content Management System User Management")

	app.StringOpt("p path", "", "Path to the CMS root directory")
	app.Version("v version", "cmsum 0.0.9")
	app.LongDesc = "https://github.com/earentir/cmsum"

	app.Before = func() {
		if cmsPath != "" {
			if _, err := os.Stat(cmsPath); os.IsNotExist(err) {
				log.Fatalf("The specified CMS path does not exist: %s", cmsPath)
			}
		}
	}

	app.Command("users", "User management commands", func(users *cli.Cmd) {
		users.Command("list", "List users", listUsers)
		users.Command("info", "Show user info", userInfo)
		users.Command("edit", "Edit user details", editUser)
	})

	app.Command("info", "Show CMS information", func(info *cli.Cmd) {
		info.Command("general", "Show general CMS information", showInfo)
		info.Command("version", "Show CMS version information", showVersion)
	})

	if err := app.Run(os.Args); err != nil {
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

func listUsers(cmd *cli.Cmd) {
	cmd.Action = func() {
		cmsType := detectCMS()
		if cmsType == "" {
			log.Fatal("Unable to detect CMS type. Make sure you're in the correct directory or specify the correct path using the -p flag.")
		}

		var err error
		if cmsType == "wordpress" {
			err = wordpress.ProcessWordPress(cmsPath)
		} else if cmsType == "joomla" {
			err = joomla.ProcessJoomla(cmsPath)
		}

		if err != nil {
			log.Printf("Error processing %s: %v", cmsType, err)
		}
	}
}

func editUser(cmd *cli.Cmd) {
	var username = cmd.StringArg("USERNAME", "", "Username of the user to edit")

	cmd.Action = func() {
		cmsType := detectCMS()
		if cmsType == "" {
			log.Fatal("Unable to detect CMS type. Make sure you're in the correct directory or specify the correct path using the -p flag.")
		}

		var err error
		if cmsType == "wordpress" {
			err = wordpress.EditUser(cmsPath, *username)
		} else if cmsType == "joomla" {
			err = joomla.EditUser(cmsPath, *username)
		}

		if err != nil {
			log.Printf("Error editing %s user: %v", cmsType, err)
		}
	}
}

func showInfo(cmd *cli.Cmd) {
	cmd.Action = func() {
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
	}
}

func showVersion(cmd *cli.Cmd) {
	cmd.Action = func() {
		cmsType := detectCMS()
		if cmsType == "" {
			log.Fatal("Unable to detect CMS type. Make sure you're in the correct directory or specify the correct path using the -p flag.")
		}

		var version string
		var err error
		if cmsType == "wordpress" {
			version, err = wordpress.GetVersion(cmsPath)
		} else if cmsType == "joomla" {
			version, err = joomla.GetVersion(cmsPath)
		}

		if err != nil {
			log.Printf("Error showing %s version: %v", cmsType, err)
		} else {
			fmt.Printf("%s Version: %s\n", cmsType, version)
		}
	}
}

func userInfo(cmd *cli.Cmd) {
	cmd.Action = func() {
		fmt.Println("User info functionality not implemented yet.")
	}
}
