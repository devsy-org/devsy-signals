package auditschema

import (
	"net/netip"
	"regexp"
	"strings"
	"testing"
)

func TestIPPatterns(t *testing.T) {
	t.Parallel()
	v4 := regexp.MustCompile("^" + ipv4Pattern + "$")
	v6 := regexp.MustCompile(ipv6Pattern())
	for _, text := range []string{
		"0.0.0.0", "255.255.255.255", "+0.0.0.0", "256.0.0.0", "01.0.0.0",
		"::", "1::", "::1", "1::2", "1:2:3:4:5:6:7:8", "1:2:3:4:5:6:7::",
		"1:2:3:4:5:6:7:8::", "1:2:3", "1::2::3", ":::1", "fe80::1%en0",
		"::192.0.2.1", "::ffff:192.0.2.1", "1:2:3:4:5:6:192.0.2.1",
		"1:2:3:4:5::192.0.2.1", "1:2:3:4:5:6::192.0.2.1", "::ffff:192.00.2.1",
	} {
		addr, err := netip.ParseAddr(text)
		want := err == nil && addr.Zone() == ""
		pattern := v4
		if strings.Contains(text, ":") {
			pattern = v6
		}
		if got := pattern.MatchString(text); got != want {
			t.Errorf("%q: IP pattern = %t, netip without zone = %t", text, got, want)
		}
	}
}
