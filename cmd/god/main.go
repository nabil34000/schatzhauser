package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	_ "github.com/mattn/go-sqlite3"

	dbpkg "github.com/aabbtree77/schatzhauser/db"
	"github.com/aabbtree77/schatzhauser/internal/config"
)

func usage() {
	base := path.Base(os.Args[0])
	fmt.Printf(`%s — administrative CLI to manage users

Usage:
  %s user get <username>
  %s user set --username <name> [--role admin|user] [--ip 1.2.3.4] [--password secret]
  %s user set --username <name> --password <newpw>
  %s user delete --username <name>

  %s users list
  %s users delete --prefix <prefix>
  %s users delete --created-between <start> <end>    (dates: YYYY-MM-DD)

Examples:
  %s user set --username alice --role admin
  %s users delete --prefix test_
  %s users delete --created-between 2024-01-01 2024-02-01
`, base, base, base, base, base, base, base, base, base, base, base)
}

func fatalf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

/*
func formatTime(v interface{}) string {
	switch t := v.(type) {
	case time.Time:
		return t.Format(time.RFC3339)
	case *time.Time:
		if t == nil {
			return ""
		}
		return t.Format(time.RFC3339)
	default:
		return fmt.Sprint(v)
	}
}
*/

func formatTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cfg, err := config.LoadConfig("config.toml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := sql.Open("sqlite3", cfg.DBPath+"?_foreign_keys=on")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(context.Background()); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	store := dbpkg.NewStore(db)

	group := os.Args[1]
	switch group {
	case "user":
		if len(os.Args) < 3 {
			usage()
			os.Exit(2)
		}
		cmd := os.Args[2]
		switch cmd {
		case "get":
			if len(os.Args) != 4 {
				fatalf("usage: user get <username>")
			}
			username := os.Args[3]
			ctx := context.Background()
			u, err := store.GetUserFullByUsername(ctx, username)
			if err != nil {
				log.Fatalf("get user: %v", err)
			}
			fmt.Printf("id: %d\nusername: %s\nrole: %s\nip: %s\ncreated_at: %s\n",
				u.ID, u.Username, u.Role, u.Ip, formatTime(u.CreatedAt))

		case "set":
			fs := flag.NewFlagSet("user set", flag.ExitOnError)
			username := fs.String("username", "", "username (required)")
			role := fs.String("role", "", "role: admin|user")
			ip := fs.String("ip", "", "ip address")
			password := fs.String("password", "", "password (required for new user)")
			fs.Parse(os.Args[3:])

			if *username == "" {
				fs.Usage()
				os.Exit(2)
			}

			// validate role if provided
			if *role != "" {
				r := strings.ToLower(*role)
				if r != "admin" && r != "user" {
					fatalf("invalid role: %s", *role)
				}
			}

			var pw string
			if *password != "" {
				b, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
				if err != nil {
					log.Fatalf("hash password: %v", err)
				}
				pw = string(b)
			}

			ctx := context.Background()

			// check if user exists
			existing, err := store.GetUserFullByUsername(ctx, *username)
			if err != nil {
				if err == sql.ErrNoRows {
					// user does not exist → create
					if pw == "" {
						fatalf("password is required for new user")
					}
					createRole := "user"
					if *role != "" {
						createRole = strings.ToLower(*role)
					}
					ipVal := ""
					if *ip != "" {
						ipVal = *ip
					}

					newUser, err := store.CreateUserWithRole(ctx, dbpkg.CreateUserWithRoleParams{
						Username:     *username,
						PasswordHash: pw,
						Ip:           ipVal,
						Role:         createRole,
					})
					if err != nil {
						log.Fatalf("create user failed: %v", err)
					}
					fmt.Printf("created: id=%d username=%s role=%s ip=%s created_at=%s\n",
						newUser.ID, newUser.Username, newUser.Role, newUser.Ip, formatTime(newUser.CreatedAt))
					return
				} else {
					log.Fatalf("get user failed: %v", err)
				}
			}

			// user exists → update
			updateRole := existing.Role
			if *role != "" {
				updateRole = strings.ToLower(*role)
			}
			updateIp := existing.Ip
			if *ip != "" {
				updateIp = *ip
			}
			updatePw := ""
			if pw != "" {
				updatePw = pw
			}

			res, err := store.UpdateUserPatch(ctx, dbpkg.UpdateUserPatchParams{
				PasswordHash: updatePw,   // empty string means unchanged
				Ip:           updateIp,   // empty string means unchanged
				Role:         updateRole, // empty string means unchanged
				Username:     *username,
			})
			if err != nil {
				log.Fatalf("user set failed: %v", err)
			}
			fmt.Printf("updated: id=%d username=%s role=%s ip=%s created_at=%s\n",
				res.ID, res.Username, res.Role, res.Ip, formatTime(res.CreatedAt))

		case "delete":
			fs := flag.NewFlagSet("user delete", flag.ExitOnError)
			username := fs.String("username", "", "username (required)")
			fs.Parse(os.Args[3:])
			if *username == "" {
				fs.Usage()
				os.Exit(2)
			}
			ctx := context.Background()
			if err := store.DeleteUserByUsername(ctx, *username); err != nil {
				log.Fatalf("delete user: %v", err)
			}
			fmt.Printf("deleted %s\n", *username)

		default:
			fatalf("unknown user subcommand: %s", cmd)
		}

	case "users":
		if len(os.Args) < 3 {
			usage()
			os.Exit(2)
		}
		cmd := os.Args[2]
		switch cmd {
		case "list":
			ctx := context.Background()
			users, err := store.ListUsers(ctx)
			if err != nil {
				log.Fatalf("list users: %v", err)
			}
			fmt.Printf("%-6s %-24s %-8s %-25s\n", "ID", "USERNAME", "ROLE", "CREATED_AT")
			for _, u := range users {
				fmt.Printf("%-6d %-24s %-8s %-25s\n", u.ID, u.Username, u.Role, formatTime(u.CreatedAt))
			}

		case "delete":
			fs := flag.NewFlagSet("users delete", flag.ExitOnError)
			prefix := fs.String("prefix", "", "delete usernames starting with this prefix")
			createdBetween := fs.Bool("created-between", false, "use created-between mode (supply two dates)")
			fs.Parse(os.Args[3:])

			ctx := context.Background()
			if *prefix != "" {
				pat := *prefix + "%"
				if err := store.DeleteUsersByPrefix(ctx, pat); err != nil {
					log.Fatalf("delete by prefix failed: %v", err)
				}
				fmt.Printf("deleted users with prefix %s\n", *prefix)
				return
			}
			if *createdBetween {
				args := fs.Args()
				if len(args) != 2 {
					fatalf("created-between requires two dates: start end (YYYY-MM-DD)")
				}
				start, err := time.Parse("2006-01-02", args[0])
				if err != nil {
					fatalf("bad start date: %v", err)
				}
				end, err := time.Parse("2006-01-02", args[1])
				if err != nil {
					fatalf("bad end date: %v", err)
				}
				params := dbpkg.DeleteUsersCreatedBetweenParams{
					Start: sql.NullTime{Time: start, Valid: true},
					End:   sql.NullTime{Time: end, Valid: true},
				}
				if err := store.DeleteUsersCreatedBetween(ctx, params); err != nil {
					log.Fatalf("delete created-between failed: %v", err)
				}
				fmt.Printf("deleted users created between %s and %s\n", start.Format("2006-01-02"), end.Format("2006-01-02"))
				return
			}
			fatalf("users delete: must provide either --prefix or --created-between (see help)")

		default:
			fatalf("unknown users subcommand: %s", cmd)
		}

	case "help", "-h", "--help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}
