package util

import (
	"fmt"
	"net/url"
	"strings"
)

// URI represents a parsed URI
type URI struct {
	Protocol string
	Host     string
	Port     int
	Path     string
	Query    string
	Fragment string
	UserInfo *url.Userinfo
	Raw      string
}

// ParseURI parses a raw URI string into a URI struct
func ParseURI(rawURI string) (*URI, error) {
	u, err := url.Parse(rawURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URI: %w", err)
	}

	protocol := strings.ToLower(u.Scheme)
	if protocol == "" {
		return nil, fmt.Errorf("protocol not specified")
	}

	host := u.Hostname()
	portStr := u.Port()
	var port int

	if portStr != "" {
		_, err := fmt.Sscanf(portStr, "%d", &port)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %s", portStr)
		}
	} else {
		switch protocol {
		case "http":
			port = 80
		case "https":
			port = 443
		case "ftp":
			port = 21
		default:
			// unknown protocol default
		}
	}

	return &URI{
		Protocol: protocol,
		Host:     host,
		Port:     port,
		Path:     u.Path,
		Query:    u.RawQuery,
		Fragment: u.Fragment,
		UserInfo: u.User,
		Raw:      rawURI,
	}, nil
}

// String returns the string representation of the URI
func (u *URI) String() string {
	return u.Raw
}
