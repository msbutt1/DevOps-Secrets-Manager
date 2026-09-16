// Package clientip works out the address of the client behind trusted reverse proxies.
//
// X-Forwarded-For is only honoured when the connection comes from a trusted proxy, and it is
// read from the right: the first address that is not itself a trusted proxy is the client.
// Headers from untrusted peers are ignored, so clients cannot choose their own address.
//
// Behind a CDN that writes the client address into its own header (Cloudflare's
// CF-Connecting-IP, Fly's Fly-Client-IP), set that header name instead: it holds one address
// the edge wrote itself, so a client cannot prepend a fake entry the way it can with
// X-Forwarded-For. It is still only read from trusted peers.
package clientip

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type contextKey struct{}

// DefaultTrustedProxies are loopback and private network ranges, where the bundled Vite dev
// server, nginx container and typical load balancers run.
var DefaultTrustedProxies = []string{"127.0.0.0/8", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"}

// Resolver finds client addresses.
type Resolver struct {
	trusted []netip.Prefix
	// header, when set, is a single-address header written by the edge (e.g. CF-Connecting-IP)
	// and is preferred over X-Forwarded-For.
	header string
}

// NewResolver parses CIDR ranges (or single addresses) of trusted proxies.
func NewResolver(trusted []string) (*Resolver, error) {
	return NewResolverWithHeader(trusted, "")
}

// NewResolverWithHeader is NewResolver with a single-address client IP header to prefer.
func NewResolverWithHeader(trusted []string, header string) (*Resolver, error) {
	r := &Resolver{header: http.CanonicalHeaderKey(strings.TrimSpace(header))}
	for _, raw := range trusted {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.Contains(raw, "/") {
			addr, err := netip.ParseAddr(raw)
			if err != nil {
				return nil, fmt.Errorf("invalid trusted proxy %q: %w", raw, err)
			}
			raw = netip.PrefixFrom(addr, addr.BitLen()).String()
		}
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy %q: %w", raw, err)
		}
		r.trusted = append(r.trusted, prefix.Masked())
	}
	return r, nil
}

func (r *Resolver) isTrusted(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, p := range r.trusted {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// ClientIP returns the client address for the request.
func (r *Resolver) ClientIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	peer = peer.Unmap()
	if !r.isTrusted(peer) {
		return peer.String()
	}

	// A single-address header from the edge, when one is configured
	if r.header != "" {
		if addr, err := netip.ParseAddr(strings.TrimSpace(req.Header.Get(r.header))); err == nil {
			return addr.Unmap().String()
		}
	}

	// Walk X-Forwarded-For from the nearest hop back towards the client
	var hops []string
	for _, header := range req.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(header, ",")...)
	}
	client := peer
	for i := len(hops) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break
		}
		client = addr.Unmap()
		if !r.isTrusted(client) {
			break
		}
	}
	return client.String()
}

// Middleware stores the client address in the request context.
func (r *Resolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ip := r.ClientIP(req)
		next.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), contextKey{}, ip)))
	})
}

// FromContext returns the address stored by Middleware, or "" when absent.
func FromContext(ctx context.Context) string {
	ip, _ := ctx.Value(contextKey{}).(string)
	return ip
}
