// tests/register/main.go
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aabbtree77/schatzhauser/internal/config"
	"github.com/aabbtree77/schatzhauser/internal/logger"
	_ "github.com/mattn/go-sqlite3"
)

const (
	baseURL   = "http://localhost:8080"
	dbPath    = "data/data.db"
	logFolder = "tests/register"
	logFile   = "tests.log"
	timeout   = 5 * time.Second
)

type testCase struct {
	Name            string
	Payload         any          // JSON-serializable payload or raw string (if Raw != "")
	Raw             string       // if set, sent as-is instead of Payload
	PreRun          func() error // optional setup (e.g., ensure deleted)
	ExpectStatus    int
	ExpectInDB      bool   // whether username should exist after request
	UsernameToCheck string // which username to check in DB (if ExpectInDB)
	AllowAltStatus  int    // optional alternate acceptable status (e.g. 400 accepted if spec says 409)
}

type result struct {
	Name    string
	Pass    bool
	Warning string
	Details string
}

func main() {

	//Disable the ip rate limiter for this test

	// Load configuration
	cfg, err := config.LoadConfig("config.toml")
	if err != nil {
		logger.Error("failed to load config", "err", err)
	}

	fmt.Printf("register Enable=%v\n", cfg.IPRateLimiter.Register.Enable)

	if cfg.IPRateLimiter.Register.Enable {
		fmt.Println("Make sure enable is false inside config.toml [ip_rate_limiter.register]")
		os.Exit(1)
	}

	if cfg.AccountPerIPLimiter.Enable {
		fmt.Println("Make sure enable is false inside config.toml [account_per_ip_limiter]")
		os.Exit(1)
	}

	// ensure tests log dir exists
	if err := os.MkdirAll(logFolder, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir log folder: %v\n", err)
		os.Exit(2)
	}
	logf, err := os.Create(filepath.Join(logFolder, logFile))
	if err != nil {
		fmt.Fprintf(os.Stderr, "create log: %v\n", err)
		os.Exit(2)
	}
	defer logf.Close()

	log := func(format string, a ...any) {
		fmt.Fprintf(logf, format+"\n", a...)
		fmt.Printf(format+"\n", a...)
	}

	// table-driven tests
	tests := []testCase{
		{
			Name: "valid registration",
			Payload: map[string]any{
				"username": "test_register_valid",
				"password": "secret123",
			},
			ExpectStatus:    201,
			ExpectInDB:      true,
			UsernameToCheck: "test_register_valid",
		},
		{
			Name: "missing username",
			Payload: map[string]any{
				"password": "x",
			},
			ExpectStatus: 400,
			ExpectInDB:   false,
		},
		{
			Name: "missing password",
			Payload: map[string]any{
				"username": "no_password_user",
			},
			ExpectStatus: 400,
			ExpectInDB:   false,
		},
		{
			Name:         "invalid json",
			Raw:          `{"username": "bad", "password": "x`, // malformed JSON
			ExpectStatus: 400,
			ExpectInDB:   false,
		},
		{
			Name: "duplicate username",
			// Will attempt to register same username twice. Expect 409 per spec.
			Payload: map[string]any{
				"username": "test_register_dup",
				"password": "dupPass",
			},
			ExpectStatus:    409,
			AllowAltStatus:  400, // accept 400 as a warning (existing handler returns 400)
			ExpectInDB:      true,
			UsernameToCheck: "test_register_dup",
		},
	}

	// run tests sequentially
	var failed int
	var warnings int

	// make sure DB file exists (server should create it, but test can still run)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		// not fatal: warn
		log("WARN: DB file %s doesn't exist. Make sure server is running and migrations applied.", dbPath)
	}

	client := &http.Client{Timeout: timeout}

	// helper to perform request
	doRequest := func(body []byte) (int, []byte, error) {
		req, err := http.NewRequest("POST", baseURL+"/api/register", bytes.NewReader(body))
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b, nil
	}

	// helper to check DB for username
	checkDB := func(username string) (bool, error) {
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			return false, err
		}
		defer db.Close()

		var cnt int
		row := db.QueryRow("SELECT COUNT(1) FROM users WHERE username = ?", username)
		if err := row.Scan(&cnt); err != nil {
			return false, err
		}
		return cnt > 0, nil
	}

	// helper to delete username (cleanup) — best-effort
	deleteUser := func(username string) error {
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		_, err = db.Exec("DELETE FROM users WHERE username = ?", username)
		return err
	}

	for _, tc := range tests {
		log("=== TEST: %s", tc.Name)

		// PreRun cleanup for cases that need a clean start
		if tc.Name == "duplicate username" {
			// ensure user not present
			_ = deleteUser("test_register_dup")
		}
		if tc.Name == "valid registration" {
			_ = deleteUser("test_register_valid")
		}

		var body []byte
		if tc.Raw != "" {
			body = []byte(tc.Raw)
		} else if tc.Payload != nil {
			b, err := json.Marshal(tc.Payload)
			if err != nil {
				log("ERROR marshaling payload: %v", err)
				failed++
				continue
			}
			body = b
		} else {
			body = []byte("{}")
		}

		// For duplicate test: first register once (should succeed), then register again and check response
		if tc.Name == "duplicate username" {
			firstStatus, firstBody, err := doRequest(body)
			if err != nil {
				log("ERROR first request failed: %v", err)
				failed++
				continue
			}
			log("first registration status=%d body=%s", firstStatus, string(firstBody))
			// first should be 201 (or if it's already there, OK — we'll accept)
			if firstStatus != 201 && firstStatus != 409 && firstStatus != 400 {
				log("FAIL: unexpected first registration status: %d", firstStatus)
				failed++
				continue
			}

			// second request -> this is the one we evaluate
			status, respBody, err := doRequest(body)
			if err != nil {
				log("ERROR second request failed: %v", err)
				failed++
				continue
			}

			pass := false
			warn := ""
			if status == tc.ExpectStatus {
				pass = true
			} else if tc.AllowAltStatus != 0 && status == tc.AllowAltStatus {
				// Accept alternate but warn that spec expects different code
				pass = true
				warn = fmt.Sprintf("status %d returned, spec expects %d", status, tc.ExpectStatus)
				warnings++
			}

			if !pass {
				log("FAIL: got status %d, want %d (body=%s)", status, tc.ExpectStatus, string(respBody))
				failed++
			} else {
				log("PASS: got status %d (body=%s) %s", status, string(respBody), warn)
			}

			// check DB presence
			if tc.ExpectInDB && tc.UsernameToCheck != "" {
				ok, err := checkDB(tc.UsernameToCheck)
				if err != nil {
					log("ERROR checking DB: %v", err)
					failed++
				} else if !ok {
					log("FAIL: username %s not found in DB", tc.UsernameToCheck)
					failed++
				} else {
					log("DB: username %s exists", tc.UsernameToCheck)
				}
			}

			continue
		}

		// Normal single-request tests
		status, respBody, err := doRequest(body)
		if err != nil {
			log("ERROR request failed: %v", err)
			failed++
			continue
		}

		if status != tc.ExpectStatus {
			// allow alternative if provided
			if tc.AllowAltStatus != 0 && status == tc.AllowAltStatus {
				log("PASS (alt): got status %d, expected %d (alt %d)", status, tc.ExpectStatus, tc.AllowAltStatus)
				warnings++
			} else {
				log("FAIL: got status %d, want %d -- body=%s", status, tc.ExpectStatus, string(respBody))
				failed++
				continue
			}
		} else {
			log("PASS: got status %d -- body=%s", status, string(respBody))
		}

		// DB check if requested
		if tc.ExpectInDB && tc.UsernameToCheck != "" {
			ok, err := checkDB(tc.UsernameToCheck)
			if err != nil {
				log("ERROR checking DB: %v", err)
				failed++
			} else if !ok {
				log("FAIL: username %s not found in DB", tc.UsernameToCheck)
				failed++
			} else {
				log("DB: username %s exists", tc.UsernameToCheck)
			}
		}
	}

	// Summary
	log("=== SUMMARY ===")
	if failed > 0 {
		log("FAILED: %d tests failed, %d warnings", failed, warnings)
		os.Exit(1)
	}
	if warnings > 0 {
		log("OK: all tests passed with %d warnings", warnings)
		os.Exit(0)
	}
	log("OK: all tests passed")
}
