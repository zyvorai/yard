package egress

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// Policy decides which outbound URLs Yard may request.
type Policy struct {
	AllowPrivate bool
	Allowlist    map[string]struct{}
	MaxBody      int64
	denials      atomic.Int64
}

// Demo allows private addresses so a local Device Agent can be reached.
func Demo() *Policy {
	return &Policy{AllowPrivate: true, MaxBody: 1 << 20}
}

// Production blocks loopback, link-local, metadata, and RFC1918 unless the
// host is listed in allow. Link-local and cloud metadata stay blocked even
// when allowlisted.
func Production(allow []string) *Policy {
	m := map[string]struct{}{}
	for _, h := range allow {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" {
			m[h] = struct{}{}
		}
	}
	return &Policy{AllowPrivate: false, Allowlist: m, MaxBody: 1 << 20}
}

// Denials is the number of URLs this policy has refused.
func (p *Policy) Denials() int64 {
	if p == nil {
		return 0
	}
	return p.denials.Load()
}

func (p *Policy) deny(err error) error {
	if p != nil {
		p.denials.Add(1)
	}
	return err
}

// ValidateOptional accepts empty, relative, and internal:// connector endpoints.
func (p *Policy) ValidateOptional(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "internal://") {
		return nil
	}
	return p.Validate(raw)
}

// Validate checks scheme and that every resolved address is allowed.
func (p *Policy) Validate(raw string) error {
	if p == nil {
		p = Demo()
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return p.deny(fmt.Errorf("invalid connector url"))
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return p.deny(fmt.Errorf("only http and https connector urls are allowed"))
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return p.deny(fmt.Errorf("connector url missing host"))
	}
	ips, err := resolve(host)
	if err != nil {
		return p.deny(err)
	}
	for _, ip := range ips {
		if err := p.allow(host, ip); err != nil {
			return p.deny(err)
		}
	}
	return nil
}

func (p *Policy) allow(host string, ip net.IP) error {
	if isMetadata(host) || isMetadataIP(ip) || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("link-local and metadata addresses are blocked")
	}
	if _, ok := p.Allowlist[strings.ToLower(host)]; ok {
		return nil
	}
	return p.checkIP(ip)
}

func resolve(host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("resolve %s: no addresses", host)
	}
	out := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, a.IP)
	}
	return out, nil
}

func (p *Policy) checkIP(ip net.IP) error {
	if ip == nil {
		return fmt.Errorf("empty address")
	}
	if isMetadataIP(ip) || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("link-local and metadata addresses are blocked")
	}
	if p != nil && p.AllowPrivate {
		return nil
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() {
		return fmt.Errorf("private and loopback addresses are blocked")
	}
	return nil
}

func isMetadata(host string) bool {
	return host == "169.254.169.254" || host == "metadata.google.internal"
}

func isMetadataIP(ip net.IP) bool {
	return ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("fd00:ec2::254"))
}

// HTTPClient dials only addresses that pass Validate and refuses redirects.
func (p *Policy) HTTPClient(timeout time.Duration, insecureTLS bool) *http.Client {
	if p == nil {
		p = Demo()
	}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := resolve(host)
			if err != nil {
				p.denials.Add(1)
				return nil, err
			}
			var chosen net.IP
			for _, ip := range ips {
				if err := p.allow(host, ip); err != nil {
					p.denials.Add(1)
					return nil, err
				}
				chosen = ip
				break
			}
			if chosen == nil {
				p.denials.Add(1)
				return nil, fmt.Errorf("no allowed address for %s", host)
			}
			d := &net.Dialer{Timeout: 10 * time.Second}
			return d.DialContext(ctx, network, net.JoinHostPort(chosen.String(), port))
		},
	}
	if insecureTLS {
		transport.TLSClientConfig = insecureTLSConfig()
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return fmt.Errorf("redirects are not allowed")
		},
	}
}

// ReadAll reads at most MaxBody bytes.
func (p *Policy) ReadAll(r io.Reader) ([]byte, error) {
	n := int64(1 << 20)
	if p != nil && p.MaxBody > 0 {
		n = p.MaxBody
	}
	return io.ReadAll(io.LimitReader(r, n))
}
