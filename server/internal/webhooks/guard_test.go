package webhooks

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

var blocked = map[string][]string{
	"loopback":    {"127.0.0.1", "127.255.255.254", "::1"},
	"link-local":  {"169.254.0.1", "169.254.169.254", "fe80::1"},
	"private":     {"10.0.0.1", "172.16.0.1", "172.31.255.255", "192.168.1.1", "fc00::1", "fd12:3456::1"},
	"cgnat":       {"100.64.0.1", "100.127.255.255"},
	"unspecified": {"0.0.0.0", "0.1.2.3", "::"},
	"multicast":   {"224.0.0.1", "239.255.255.255", "ff02::1"},
	"ietf":        {"192.0.0.8"},
	"test-net":    {"192.0.2.1", "198.51.100.1", "203.0.113.1", "2001:db8::1"},
	"benchmark":   {"198.18.0.1", "198.19.255.255"},
	"reserved":    {"240.0.0.1", "255.255.255.255"},
	"v4-mapped":   {"::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:169.254.169.254", "::ffff:192.168.0.1", "::ffff:100.64.0.1"},
	"6to4":        {"2002:7f00:1::1", "2002:0a00:1::1", "2002:c0a8:1::1"},
	"nat64":       {"64:ff9b::7f00:1", "64:ff9b::a00:1", "64:ff9b::a9fe:a9fe"},
	"teredo":      {"2001::1"},
	"v4-compat":   {"::7f00:1"},
}

var allowed = []string{"93.184.216.34", "8.8.8.8", "1.1.1.1", "172.15.0.1", "172.32.0.1", "100.63.255.255", "100.128.0.1", "2606:4700:4700::1111", "2a00:1450:4001::1", "::ffff:8.8.8.8"}

func TestInternalRanges(t *testing.T) {
	for name, addrs := range blocked {
		for _, a := range addrs {
			if !internal(netip.MustParseAddr(a)) {
				t.Errorf("%s %s must be refused", name, a)
			}
			if err := guardDial("tcp", net.JoinHostPort(a, "443"), nil); err == nil {
				t.Errorf("%s %s must not be dialled", name, a)
			}
		}
	}
	for _, a := range allowed {
		if internal(netip.MustParseAddr(a)) {
			t.Errorf("public %s must be accepted", a)
		}
		if err := guardDial("tcp", net.JoinHostPort(a, "443"), nil); err != nil {
			t.Errorf("public %s refused at dial: %v", a, err)
		}
	}
	if guardDial("tcp", "not-an-ip:443", nil) == nil {
		t.Error("an unparsed host must fail closed")
	}
}

func TestCheckURLResolvedNames(t *testing.T) {
	old := lookupIP
	t.Cleanup(func() { lookupIP = old })
	for name, addrs := range blocked {
		for _, a := range addrs {
			ip := net.ParseIP(a)
			lookupIP = func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("93.184.216.34"), ip}, nil } // one public, one internal answer
			if err := CheckURL("https://hook.example.org/h"); !errors.Is(err, ErrInvalid) {
				t.Errorf("%s: a name resolving to %s must be refused, got %v", name, a, err)
			}
			host := a
			if netip.MustParseAddr(a).Is6() {
				host = "[" + a + "]"
			}
			if err := CheckURL("https://" + host + "/h"); !errors.Is(err, ErrInvalid) {
				t.Errorf("%s: literal %s must be refused, got %v", name, a, err)
			}
		}
	}
	for _, a := range allowed {
		lookupIP = func(string) ([]net.IP, error) { return []net.IP{net.ParseIP(a)}, nil }
		if err := CheckURL("https://hook.example.org/h"); err != nil {
			t.Errorf("public %s refused: %v", a, err)
		}
	}
}

// The dial itself is checked (a name that resolves inward at connect time, the DNS-rebinding case) and redirects are not followed.
func TestClientDialsOnlyPublicAndNeverRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	t.Cleanup(srv.Close)
	c := New(nil).Client
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	for _, target := range []string{srv.URL, "http://localhost:" + port} { // literal loopback, and a name that resolves to it at dial time
		if resp, err := c.Get(target); err == nil {
			resp.Body.Close()
			t.Errorf("dial to %s must be refused", target)
		}
	}
	if err := c.CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("redirects must not be followed, got %v", err)
	}
}
