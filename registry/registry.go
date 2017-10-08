// Package registry contains client primitives to interact with a remote Docker registry.
package registry

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
)

// GitClient returns an git client structure
func GitClient() *git.Client {
	return &git.Client{}
}

// NewTransport returns a new HTTP transport. If tlsConfig is nil, it uses the
// default TLS configuration.
func NewTransport(tlsConfig *tls.Config) *http.Transport {
	if tlsConfig == nil {
		tlsConfig = tlsconfig.ServerDefault()
	}

	direct := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}

	base := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		Dial:                direct.Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		// TODO(dmcgowan): Call close idle connections when complete and use keep alive
		DisableKeepAlives: true,
	}

	proxyDialer, err := sockets.DialerFromEnvironment(direct)
	if err == nil {
		base.Dial = proxyDialer.Dial
	}
	return base
}
