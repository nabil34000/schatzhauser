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

const (
	baseURL = "http://localhost:8080"
)

func must(err error) {
	if err != nil {
		fmt.Println("ERROR:", err)
		os.Exit(1)
	}
}

func registerUser(username, password string) {
	payload := map[string]string{"username": username, "password": password}
	data, _ := json.Marshal(payload)
	resp, err := http.Post(baseURL+"/api/register", "application/json", bytes.NewReader(data))
	must(err)
	defer resp.Body.Close()
	if resp.StatusCode != 201 && resp.StatusCode != 409 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("register failed: status=%d body=%s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}
}

func doLogin(username, password string) (int, string, error) {
	payload := map[string]string{"username": username, "password": password}
	data, _ := json.Marshal(payload)
	resp, err := http.Post(baseURL+"/api/login", "application/json", bytes.NewReader(data))
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), nil
}

func main() {

	// parameters must match [ip_rate_limiter.login] in config.toml

	// Load configuration
	cfg, err := config.LoadConfig("config.toml")
	if err != nil {
		logger.Error("failed to load config", "err", err)
	}

	if !cfg.IPRateLimiter.Login.Enable {
		fmt.Println("Make sure enable is true inside config.toml [ip_rate_limiter.login]")
		os.Exit(1)
	}

	if cfg.AccountPerIPLimiter.Enable {
		fmt.Println("Make sure enable is false inside config.toml [account_per_ip_limiter]")
		os.Exit(1)
	}

	threshold := cfg.IPRateLimiter.Login.MaxRequests
	window := time.Duration(cfg.IPRateLimiter.Login.WindowMS) * time.Millisecond

	user := fmt.Sprintf("ratelimit_test_%d", time.Now().UnixNano())
	//user := "test_register_valid"
	pass := "secret123"
	registerUser(user, pass)

	// Rapid invalid password attempts to cause 401 until rate limited
	total := 20
	first429At := -1
	for i := 0; i < total; i++ {
		//status, body, err := doLogin(user, "badpassword")
		status, body, err := doLogin(user, "secret123")

		if err != nil {
			fmt.Println("ERROR login:", err)
			os.Exit(1)
		}
		fmt.Printf("attempt %d -> status=%d body=%s\n", i+1, status, body)
		if status == 429 && first429At == -1 {
			first429At = i + 1
		}
		// tiny sleep to simulate burst but keep in same second
		time.Sleep(50 * time.Millisecond)
	}

	if first429At == -1 {
		fmt.Println("FAIL: never received 429, rate limiter did not trigger")
		os.Exit(1)
	}
	fmt.Printf("INFO: first 429 at attempt %d\n", first429At)

	// Expect limiter to have triggered at or after threshold+1
	if first429At <= threshold {
		fmt.Printf("FAIL: got 429 too early at %d (threshold=%d)\n", first429At, threshold)
		os.Exit(1)
	}

	// wait for window to expire
	fmt.Println("waiting window to expire...")
	time.Sleep(window + 200*time.Millisecond)

	// Now should be allowed again (401 for wrong password)
	status, body, err := doLogin(user, "badpassword")
	must(err)
	fmt.Printf("after wait -> status=%d body=%s\n", status, body)
	if status == 429 {
		fmt.Println("FAIL: still rate limited after window")
		os.Exit(1)
	}

	fmt.Println("PASS: rate limit behaved as expected")
}
