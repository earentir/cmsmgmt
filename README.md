# cmsmgmt

Content Management System Management (cmsmgmt) is a command-line tool written in Go for inspecting and managing local installations of WordPress and Joomla. It can detect the CMS type in a given directory, extract the database configuration from the CMS configuration file, connect to the database, list user accounts, edit user details, and show general or version information.

## Features

- **Automatic CMS detection** – Point `cmsmgmt` at the root of your CMS installation using the `-p`/`--path` flag (or run it in the CMS directory), and it will determine whether you are working with WordPress or Joomla by looking for `wp-config.php` or `configuration.php`.
- **Database configuration parsing** – Reads your CMS configuration to determine connection details for MySQL/PostgreSQL (Joomla) or MySQL (WordPress), including host, port, username, password and database name.
- **List users** – Enumerates all user accounts in your CMS. For WordPress it reports the username, e-mail, role and other metadata; for Joomla it shows ID, username, name, email and roles.
- **Edit users** – Allows you to update user information (name and e-mail) for both WordPress and Joomla. Run `cmsmgmt users edit <username>` and follow the prompts.
- **CMS information** – Displays general information about the CMS and version number. The `info general` command prints the database name, database user and detected table prefixes. `info version` prints the WordPress or Joomla version (and release for Joomla).
- **Cross-database support** – Joomla installations can be backed by MySQL or PostgreSQL. WordPress support currently assumes MySQL.

## Installation

This project is written in Go. To build from source you need Go ≥ 1.24:

```bash
git clone https://github.com/earentir/cmsmgmt.git
cd cmsmgmt
go build -o cmsmgmt
```

Alternatively, you can install the latest version into your `$GOBIN` using:

```bash
go install github.com/earentir/cmsmgmt@latest
```

The resulting binary can be copied anywhere on your `PATH`.

## Usage

Run `cmsmgmt --help` to see top-level usage. The basic pattern is:

```bash
cmsmgmt [--path /path/to/cms] <command> [subcommand] [flags]
```

If `--path` is omitted, `cmsmgmt` assumes the current working directory is the root of your CMS installation.

### List users

```bash
# From within the CMS root directory
cmsmgmt users list

# Or specify the CMS path explicitly
cmsmgmt --path /var/www/html users list
```

### Show CMS information

```bash
# Display general information such as DB name, DB user and table prefixes
cmsmgmt info general

# Show CMS version (and release for Joomla)
cmsmgmt info version
```

### Edit a user

```bash
# Edit the user with username "admin"
cmsmgmt users edit admin
```

When you edit a user, `cmsmgmt` will connect to the database and update the user's name and e-mail address.

## Roadmap

Future enhancements may include:

- Changing passwords for Joomla and WordPress users.
- Additional CMS support beyond WordPress and Joomla.
- More detailed reporting on CMS configuration and security settings.

## Authors

- [@earentir](https://www.github.com/earentir)

## License

This project follows the Linux Kernel licence (GPL v2). If you require another open licence, please open an issue and we will try to accommodate it.
