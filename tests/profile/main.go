package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"time"

	"github.com/aabbtree77/schatzhauser/internal/config"
	"github.com/aabbtree77/schatzhauser/internal/logger"
)

func must(err error) {
	if err != nil {
		fmt.Println("ERROR:", err)
		os.Exit(1)
	}
}

func registerAndLogin() (*http.Client, string) {
	username := fmt.Sprintf("profile_test_%d", time.Now().UnixNano())
	password := "secret123"

	// register
	payload := map[string]string{"username": username, "password": password}
	data, _ := json.Marshal(payload)
	resp, err := http.Post("http://localhost:8080/api/register", "application/json", bytes.NewReader(data))
	must(err)
	resp.Body.Close()
	if resp.StatusCode != 201 && resp.StatusCode != 409 {
		fmt.Println("Failed to register user:", resp.StatusCode)
		os.Exit(1)
	}

	// login
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp, err = client.Post("http://localhost:8080/api/login", "application/json", bytes.NewReader(data))
	must(err)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Println("Failed to login:", resp.StatusCode, string(body))
		os.Exit(1)
	}

	// verify cookie
	u, _ := url.Parse("http://localhost:8080/")
	cookies := client.Jar.Cookies(u)
	fmt.Println("Cookies after login:", cookies)
	if len(cookies) == 0 {
		fmt.Println("Session cookie not set after login")
		os.Exit(1)
	}

	return client, username
}

func runTestProfile() bool {
	client, username := registerAndLogin()

	// access protected /profile
	resp, err := client.Get("http://localhost:8080/api/profile")
	must(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if resp.StatusCode != 200 || !bytes.Contains([]byte(bodyStr), []byte(username)) {
		fmt.Printf("FAIL: profile with cookie: status=%d, body=%s\n", resp.StatusCode, bodyStr)
		return false
	}
	fmt.Printf("PASS: profile with cookie: status=%d, body=%s\n", resp.StatusCode, bodyStr)

	// access without cookie
	resp2, err := http.Get("http://localhost:8080/api/profile")
	must(err)
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != 401 {
		fmt.Printf("FAIL: profile without cookie: status=%d, body=%s\n", resp2.StatusCode, string(body2))
		return false
	}
	fmt.Printf("PASS: profile without cookie: got 401 as expected\n")

	return true
}

func main() {

	//Disable the ip rate limiter for this test

	// Load configuration
	cfg, err := config.LoadConfig("config.toml")
	if err != nil {
		logger.Error("failed to load config", "err", err)
	}

	if cfg.IPRateLimiter.Register.Enable || cfg.IPRateLimiter.Login.Enable || cfg.IPRateLimiter.Profile.Enable || cfg.IPRateLimiter.Logout.Enable {
		fmt.Println("Make sure enable is false inside config.toml [ip_rate_limiter.register]")
		fmt.Println("Make sure enable is false inside config.toml [ip_rate_limiter.login]")
		fmt.Println("Make sure enable is false inside config.toml [ip_rate_limiter.profile]")
		fmt.Println("Make sure enable is false inside config.toml [ip_rate_limiter.logout]")
		os.Exit(1)
	}

	if cfg.AccountPerIPLimiter.Enable {
		fmt.Println("Make sure enable is false inside config.toml [account_per_ip_limiter]")
		os.Exit(1)
	}

	passed := 0
	if runTestProfile() {
		passed++
	}

	fmt.Printf("=== SUMMARY: %d/1 passed ===\n", passed)
	if passed != 1 {
		os.Exit(1)
	}
}
