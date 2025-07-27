# cmsum
CMS Management
A tool to manage CMS in the current path
Supports:
- Wordpress
- Joomla


## Features

- User Managemt
    - List Users/Roles
    - Edit User
- CMS Information
    - Detect CMS Type
    - General Information
    - Version Information

## Usage/Examples

### WordPress
```bash
[earentir@Athena www]$ cmsum

DB Name: website_db
DB User: website_user
Identified WordPress table prefixes: wp
WordPress Users for prefix 'wp':
ID: 1, Username: administrator, Email: administrator@domain.tld, Role: Administrator, First Name: , Last Name: , Nickname:
```


### Joomla
```bash
[earentir@Athena www]$ cmsum

 ~/cmsum
DB Name: website_db
DB User: website_user

Identified Joomla table prefixes: rq5bl

Identified Joomla users:
ID: 739, Username: administrator, Roles: Super Users, Name: Super User, Email: info@domain.tld
ID: 740, Username: admin, Roles: Super Users, Name: Admin, Email: support@domain.tld
```


## Roadmap

- Change User Password

- Return CMS Version

## Dependancies & Documentation

[![Go Mod](https://img.shields.io/github/go-mod/go-version/earentir/cmsum)]()

[![Go Reference](https://pkg.go.dev/badge/github.com/earentir/cmsum.svg)](https://pkg.go.dev/github.com/earentir/cmsum)

[![Dependancies](https://img.shields.io/librariesio/github/earentir/cmsum)]()
## Authors

- [@earentir](https://www.github.com/earentir)


## License

I will always follow the Linux Kernel License as primary, if you require any other OPEN license please let me know and I will try to accomodate it.

[![License](https://img.shields.io/github/license/earentir/gitearelease)](https://opensource.org/license/gpl-2-0)
