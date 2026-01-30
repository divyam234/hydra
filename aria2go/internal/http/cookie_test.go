package http

import (
	"net/http"
	"os"
	"testing"
)

func TestLoadCookiesFromNetscape(t *testing.T) {
	// Create a temporary cookie file
	content := `# Netscape HTTP Cookie File
.google.com	TRUE	/	FALSE	2147483647	NID	12345
example.com	FALSE	/path	TRUE	2147483647	session	abc def
`
	tmpfile, err := os.CreateTemp("", "cookies.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	jar, err := LoadCookiesFromNetscape(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadCookiesFromNetscape failed: %v", err)
	}

	// Verify first cookie
	u1, _ := http.NewRequest("GET", "http://www.google.com/", nil)
	cookies1 := jar.Cookies(u1.URL)
	foundNID := false
	for _, c := range cookies1 {
		if c.Name == "NID" && c.Value == "12345" {
			foundNID = true
			break
		}
	}
	if !foundNID {
		t.Error("NID cookie not found for google.com")
	}

	// Verify second cookie (secure and path)
	u2, _ := http.NewRequest("GET", "https://example.com/path/file", nil)
	cookies2 := jar.Cookies(u2.URL)
	foundSession := false
	for _, c := range cookies2 {
		if c.Name == "session" && c.Value == "abc def" {
			foundSession = true
			break
		}
	}
	if !foundSession {
		t.Error("session cookie not found for example.com/path")
	}

	// Should not find secure cookie on http
	u3, _ := http.NewRequest("GET", "http://example.com/path", nil)
	cookies3 := jar.Cookies(u3.URL)
	for _, c := range cookies3 {
		if c.Name == "session" {
			t.Error("Found secure cookie on http request")
		}
	}
}
