// Package util provides utility functions for the CLI Proxy API server.
// It includes helper functions for proxy configuration, HTTP client setup,
// log level management, and other common operations used across the application.
package util

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/config"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

// SetProxy configures the provided HTTP client with proxy settings from the configuration.
// It supports SOCKS5, HTTP, and HTTPS proxies. The function modifies the client's transport
// to route requests through the configured proxy server.
func SetProxy(cfg *config.SDKConfig, httpClient *http.Client) *http.Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	if cfg == nil {
		return httpClient
	}

	proxyStr := strings.TrimSpace(cfg.ProxyURL)
	if proxyStr == "" {
		return httpClient
	}

	proxyURL, errParse := url.Parse(proxyStr)
	if errParse != nil {
		log.Errorf("parse proxy URL failed: %v", errParse)
		return httpClient
	}

	var transport *http.Transport
	switch proxyURL.Scheme {
	case "socks5":
		var proxyAuth *proxy.Auth
		if proxyURL.User != nil {
			username := proxyURL.User.Username()
			password, _ := proxyURL.User.Password()
			proxyAuth = &proxy.Auth{User: username, Password: password}
		}
		dialer, errSOCKS5 := proxy.SOCKS5("tcp", proxyURL.Host, proxyAuth, proxy.Direct)
		if errSOCKS5 != nil {
			log.Errorf("create SOCKS5 dialer failed: %v", errSOCKS5)
			return httpClient
		}
		transport = &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		}
	case "http", "https":
		transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	default:
		return httpClient
	}

	httpClient.Transport = transport
	return httpClient
}
