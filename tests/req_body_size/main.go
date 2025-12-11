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
	Route      string
	Method     string
	Body       any
	ExpectCode int
}

func main() {

	//Disable the ip rate limiters and max account limiter for this test

	// Load configuration
	cfg, err := config.LoadConfig("config.toml")
	if err != nil {
		logger.Error("failed to load config", "err", err)
	}

	if cfg.IPRateLimiter.Register.Enable || cfg.IPRateLimiter.Login.Enable || cfg.AccountPerIPLimiter.Enable {
		fmt.Println("Make sure enable is false inside config.toml [ip_rate_limiter.register]")
		fmt.Println("Make sure enable is false inside config.toml [ip_rate_limiter.login]")
		fmt.Println("Make sure enable is false inside config.toml [account_per_ip_limiter]")
		os.Exit(1)
	}

	if !cfg.RBodySizeLimiter.Register.Enable || !cfg.RBodySizeLimiter.Login.Enable {
		fmt.Println("Make sure enable is true inside config.toml [rbody_size_limiter.register]")
		fmt.Println("Make sure enable is true inside config.toml [rbody_size_limiter.login]")
		os.Exit(1)
	}

	// unique username per run
	username := fmt.Sprintf("u1_%d", time.Now().UnixNano())

	cases := []TestCase{
		{
			Name:       "Register small payload",
			Route:      "/register",
			Method:     "POST",
			Body:       map[string]string{"username": username, "password": "p1"},
			ExpectCode: http.StatusCreated, // 201
		},
		{
			Name:       "Login small payload",
			Route:      "/login",
			Method:     "POST",
			Body:       map[string]string{"username": username, "password": "p1"},
			ExpectCode: http.StatusOK, // 200
		},
		{
			Name:       "Register empty payload",
			Route:      "/register",
			Method:     "POST",
			Body:       map[string]string{},
			ExpectCode: http.StatusBadRequest,
		},
		{
			Name:       "Login empty payload",
			Route:      "/login",
			Method:     "POST",
			Body:       map[string]string{},
			ExpectCode: http.StatusBadRequest,
		},
		{
			Name:       "Register too large payload",
			Route:      "/register",
			Method:     "POST",
			Body:       generateLargeJSON(10 << 10), // 10 KB
			ExpectCode: http.StatusRequestEntityTooLarge,
		},
		{
			Name:       "Login too large payload",
			Route:      "/login",
			Method:     "POST",
			Body:       generateLargeJSON(10 << 10), // 10 KB
			ExpectCode: http.StatusRequestEntityTooLarge,
		},
	}

	allPassed := true

	for _, tc := range cases {
		fmt.Printf("=== %s ===\n", tc.Name)
		status, respBody, err := doRequest(tc.Method, "http://localhost:8080/api"+tc.Route, tc.Body)
		if err != nil {
			fmt.Printf("ERROR: %v\n\n", err)
			allPassed = false
			continue
		}

		pass := status == tc.ExpectCode
		if !pass {
			allPassed = false
		}

		fmt.Printf("[%s] expected=%d got=%d\n", map[bool]string{true: "PASS", false: "FAIL"}[pass], tc.ExpectCode, status)
		if len(respBody) > 0 {
			fmt.Printf("Response body: %s\n", string(respBody))
		}
		fmt.Println()
	}

	if allPassed {
		fmt.Println("ALL OK ✅")
	} else {
		fmt.Println("SOME TESTS FAILED ❌")
	}
}

// doRequest encodes body to JSON and sends request.
func doRequest(method, url string, body any) (int, []byte, error) {
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		buf = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, respBody, nil
}

// generateLargeJSON creates a map with a "dummy" field of approximately size bytes.
// It pads with 'a' characters.
func generateLargeJSON(size int) map[string]string {
	s := bytes.Repeat([]byte("a"), size)
	return map[string]string{"dummy": string(s)}
}
