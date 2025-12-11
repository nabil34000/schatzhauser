package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aabbtree77/schatzhauser/internal/config"
	"github.com/aabbtree77/schatzhauser/internal/logger"
)

// -------- CONFIG --------

const (
	registerURL = "http://localhost:8080/api/register"
)

func freshIP() string {
	return fmt.Sprintf("203.0.113.%d", time.Now().UnixNano()%250+1)
}

func register(username string, testIP string) (int, string) {

	body, _ := json.Marshal(map[string]string{
		"username": username,
		"password": "secret123",
	})

	req, err := http.NewRequest(http.MethodPost, registerURL, bytes.NewReader(body))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	//X-Test-IP is needed to avoid server's register handler
	//receiving ip=127.0.0.1 no matter what testIP is.
	//req.Header.Set("X-Forwarded-For", testIP)
	req.Header.Set("X-Test-IP", testIP)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("❌ request failed:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)

	return resp.StatusCode, buf.String()
}

func main() {

	//Warning: this test adds the number of max_accounts set inside config.toml [account_per_ip_limiter]
	//directly to data.db, and it does not delete them.

	// Load configuration
	cfg, err := config.LoadConfig("config.toml")
	if err != nil {
		logger.Error("failed to load config", "err", err)
	}

	if cfg.IPRateLimiter.Register.Enable {
		fmt.Println("Make sure enable is false inside config.toml [ip_rate_limiter.register]")
		os.Exit(1)
	}

	if !cfg.AccountPerIPLimiter.Enable {
		fmt.Println("Make sure enable is true inside config.toml [account_per_ip_limiter]")
		os.Exit(1)
	}

	testIP := freshIP()

	maxAllowed := cfg.AccountPerIPLimiter.MaxAccounts
	totalTries := cfg.AccountPerIPLimiter.MaxAccounts + 5

	fmt.Println("== account-per-IP limiter black-box test ==")
	fmt.Println("endpoint:", registerURL)
	fmt.Println("test IP: ", testIP)
	fmt.Println("expected max:", maxAllowed)
	fmt.Println()

	var ok, blocked int

	for i := 1; i <= totalTries; i++ {
		username := fmt.Sprintf("iptest_%d_%d", time.Now().Unix(), i)
		status, body := register(username, testIP)

		switch status {
		case http.StatusCreated, http.StatusOK:
			ok++
			fmt.Printf("✅ %d: created account %q\n", i, username)
		case http.StatusTooManyRequests:
			blocked++
			fmt.Printf("🚫 %d: blocked (429)\n", i)
		default:
			fmt.Printf(
				"❌ %d: unexpected status %d\nresponse:\n%s\n",
				i, status, body,
			)
			os.Exit(1)
		}
	}

	fmt.Println()
	fmt.Println("== result ==")
	fmt.Printf("created: %d (expected %d)\n", ok, maxAllowed)
	fmt.Printf("blocked: %d (expected %d)\n", blocked, totalTries-maxAllowed)

	if ok != maxAllowed || blocked != totalTries-maxAllowed {
		fmt.Println("❌ TEST FAILED")
		os.Exit(1)
	}

	fmt.Println("✅ TEST PASSED")
}
