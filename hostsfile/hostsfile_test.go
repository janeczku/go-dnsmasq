package hosts

import (
	"fmt"
	"net"
	"runtime"
	"strings"
	"testing"
)

const ipv4Pass = `
127.0.0.1
127.0.1.1
10.200.30.50
99.99.99.99
999.999.999.999
0.1.1.0
`

const ipv4Fail = `
1234.1.1.1
123.5.6
12.12
76.76.67.67.45
`

const domain = "localhost"
const ip = "127.0.0.1"
const ipv6 = false

func Diff(expected, actual string) string {
	return fmt.Sprintf(`
---- Expected ----
%s
----- Actual -----
%s
`, expected, actual)
}

// Contains returns true if this Hostlist has the specified Hostname
func (h *hostlist) Contains(b *hostname) bool {
	for _, a := range *h {
		if a.Equal(b) {
			return true
		}
	}
	return false
}

func TestParseLine(t *testing.T) {
	var hosts hostlist

	// Blank line
	hosts = parseLine("")
	if len(hosts) > 0 {
		t.Error("Expected to find zero hostnames")
	}

	// Comment
	hosts = parseLine("# The following lines are desirable for IPv6 capable hosts")
	if len(hosts) > 0 {
		t.Error("Expected to find zero hostnames")
	}

	// Single word comment
	hosts = parseLine("#blah")
	if len(hosts) > 0 {
		t.Error("Expected to find zero hostnames")
	}

	hosts = parseLine("#66.33.99.11              test.domain.com")
	if len(hosts) > 0 {
		t.Error("Expected to find zero hostnames when line is commented out")
	}

	// Not Commented stuff
	hosts = parseLine("255.255.255.255 broadcasthost test.domain.com	domain.com")
	if !hosts.Contains(newHostname("broadcasthost", net.ParseIP("255.255.255.255"), false)) ||
		!hosts.Contains(newHostname("test.domain.com", net.ParseIP("255.255.255.255"), false)) ||
		!hosts.Contains(newHostname("domain.com", net.ParseIP("255.255.255.255"), false)) ||
		len(hosts) != 3 {
		t.Error("Expected to find broadcasthost, domain.com, and test.domain.com")
	}

	// Ipv6 stuff
	hosts = hostess.parseLine("::1             localhost")
	if !hosts.Contains(newHostname("localhost", net.ParseIP("::1"), true)) ||
		len(hosts) != 1 {
		t.Error("Expected to find localhost ipv6 (enabled)")
	}

	hosts = hostess.parseLine("ff02::1 ip6-allnodes")
	if !hosts.Contains(newHostname("ip6-allnodes", net.ParseIP("ff02::1"), true)) ||
		len(hosts) != 1 {
		t.Error("Expected to find ip6-allnodes ipv6 (enabled)")
	}
}

func TestHostname(t *testing.T) {
	h := newHostname(domain, net.ParseIP(ip), ipv6)

	if h.domain != domain {
		t.Errorf("Domain should be %s", domain)
	}
	if !h.ip.Equal(net.ParseIP(ip)) {
		t.Errorf("IP should be %s", ip)
	}
	if h.ipv6 != enabled {
		t.Errorf("Enabled should be %t", enabled)
	}
}
