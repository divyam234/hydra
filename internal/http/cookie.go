package http

import (
	"bufio"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// LoadCookiesFromNetscape loads cookies from a Netscape/Mozilla formatted file
func LoadCookiesFromNetscape(path string) (*cookiejar.Jar, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Use nil options (no public suffix list) to avoid external dependencies
	// This implies we trust the cookies in the file.
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Try tab separation first (standard)
		parts := strings.Split(line, "\t")
		// Fallback to fields if not enough tabs (some tools use spaces)
		if len(parts) < 7 {
			parts = strings.Fields(line)
		}

		if len(parts) < 7 {
			continue
		}

		domain := parts[0]
		// flag := parts[1] // include_subdomains
		pathStr := parts[2]
		secure := strings.ToUpper(parts[3]) == "TRUE"
		expires, _ := strconv.ParseInt(parts[4], 10, 64)
		name := parts[5]
		value := parts[6]

		// If used spaces, value might be split, join the rest
		if len(parts) > 7 {
			value = strings.Join(parts[6:], " ")
		}

		cookie := &http.Cookie{
			Name:    name,
			Value:   value,
			Path:    pathStr,
			Domain:  strings.TrimPrefix(domain, "."), // Normalize domain for cookie
			Expires: time.Unix(expires, 0),
			Secure:  secure,
		}

		// Construct a URL to associate the cookie with in the jar
		scheme := "http"
		if secure {
			scheme = "https"
		}

		// Ensure domain is a valid host (strip leading dot)
		host := strings.TrimPrefix(domain, ".")
		if host == "" {
			continue
		}

		u := &url.URL{
			Scheme: scheme,
			Host:   host,
			Path:   pathStr,
		}

		// SetCookies ignores the return value
		jar.SetCookies(u, []*http.Cookie{cookie})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return jar, nil
}
