package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/aabbtree77/schatzhauser/internal/config"
	"github.com/aabbtree77/schatzhauser/internal/logger"
)

type TestCase struct {
	Name       string
	Payload    any
	ExpectCode int
	ExpectBody string // substring match
}

func contains(s, sub string) bool {
	return bytes.Contains([]byte(s), []byte(sub))
}

func runTest(tc TestCase) bool {
	var data []byte
	switch v := tc.Payload.(type) {
	case string: // raw JSON
		data = []byte(v)
	default:
		data, _ = json.Marshal(tc.Payload)
	}

	resp, err := http.Post("http://localhost:8080/api/login", "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Printf("ERROR %s: %v\n", tc.Name, err)
		return false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if resp.StatusCode != tc.ExpectCode || !contains(bodyStr, tc.ExpectBody) {
		fmt.Printf("FAIL %s: status=%d, expected=%d, body=%s\n", tc.Name, resp.StatusCode, tc.ExpectCode, bodyStr)
		return false
	}

	fmt.Printf("PASS: %s -- got status %d, body=%s\n", tc.Name, resp.StatusCode, bodyStr)
	return true
}

// registerTempUser creates a user for login tests
func registerTempUser(username, password string) error {
	payload := map[string]string{
		"username": username,
		"password": password,
	}
	data, _ := json.Marshal(payload)

	resp, err := http.Post("http://localhost:8080/api/register", "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 409 { // 409 ok if user exists
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to register user: status=%d body=%s", resp.StatusCode, string(body))
	}

	return nil
}

func main() {

	//Disable the ip rate limiter for this test

	// Load configuration
	cfg, err := config.LoadConfig("config.toml")
	if err != nil {
		logger.Error("failed to load config", "err", err)
	}

	if cfg.IPRateLimiter.Register.Enable || cfg.IPRateLimiter.Login.Enable {
		fmt.Println("Make sure enable is false inside config.toml [ip_rate_limiter.register]")
		fmt.Println("Make sure enable is false inside config.toml [ip_rate_limiter.login]")
		os.Exit(1)
	}

	if cfg.AccountPerIPLimiter.Enable {
		fmt.Println("Make sure enable is false inside config.toml [account_per_ip_limiter]")
		os.Exit(1)
	}

	username := fmt.Sprintf("login_test_%d", time.Now().UnixNano())
	password := "secret123"

	// ensure temp user exists
	if err := registerTempUser(username, password); err != nil {
		fmt.Println("User registration failed:", err)
		os.Exit(1)
	}

	tests := []TestCase{
		{
			Name:       "valid login",
			Payload:    map[string]string{"username": username, "password": password},
			ExpectCode: 200,
			ExpectBody: fmt.Sprintf(`"username":"%s"`, username),
		},
		{
			Name:       "wrong password",
			Payload:    map[string]string{"username": username, "password": "wrongpass"},
			ExpectCode: 401,
			ExpectBody: "invalid credentials",
		},
		{
			Name:       "non-existent user",
			Payload:    map[string]string{"username": "doesnotexist", "password": "whatever"},
			ExpectCode: 401,
			ExpectBody: "invalid credentials",
		},
		{
			Name:       "invalid json",
			Payload:    "{bad json}",
			ExpectCode: 400,
			ExpectBody: "invalid json",
		},
	}

	passed := 0
	for _, tc := range tests {
		if runTest(tc) {
			passed++
		}
	}

	fmt.Printf("=== SUMMARY: %d/%d passed ===\n", passed, len(tests))
	if passed != len(tests) {
		os.Exit(1)
	}
}
