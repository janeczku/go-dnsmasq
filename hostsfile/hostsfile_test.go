package hosts

import (
	"fmt"
	"net"
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
const wildcard = false

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

func TestEquality(t *testing.T) {
	var host1 *hostname
	var host2 *hostname

	host1 = newHostname("hello", net.ParseIP("255.255.255.255"), false, false);
	host2 = newHostname("hello", net.ParseIP("255.255.255.255"), false, false);
	if !host1.Equal(host2) {
		t.Error("Hosts are expected equal, got: ", host1, host2);
	}

	host2 = newHostname("hello2", net.ParseIP("255.255.255.255"), false, false);
	if host1.Equal(host2) {
		t.Error("Hosts are expected different, got: ", host1, host2);
	}

	host2 = newHostname("hello1", net.ParseIP("255.255.255.254"), false, false);
	if host1.Equal(host2) {
		t.Error("Hosts are expected different, got: ", host1, host2);
	}

	host2 = newHostname("hello1", net.ParseIP("255.255.255.255"), true, false);
	if host1.Equal(host2) {
		t.Error("Hosts are expected different, got: ", host1, host2);
	}

	host2 = newHostname("hello1", net.ParseIP("255.255.255.255"), false, true);
	if host1.Equal(host2) {
		t.Error("Hosts are expected different, got: ", host1, host2);
	}

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

	var err error;
	err = hosts.add(newHostname("aaa", net.ParseIP("192.168.0.1"), false, false));
	if err != nil {
		t.Error("Did not expect error on first hostname");
	}
	err = hosts.add(newHostname("aaa", net.ParseIP("192.168.0.1"), false, false));
	if err == nil {
		t.Error("Expected error on duplicate host");
	}

	// Not Commented stuff
	hosts = parseLine("192.168.0.1 broadcasthost test.domain.com	domain.com")
	if !hosts.Contains(newHostname("broadcasthost", net.ParseIP("192.168.0.1"), false, false)) ||
		!hosts.Contains(newHostname("test.domain.com", net.ParseIP("192.168.0.1"), false, false)) ||
		!hosts.Contains(newHostname("domain.com", net.ParseIP("192.168.0.1"), false, false)) ||
		len(hosts) != 3 {
		t.Error("Expected to find broadcasthost, domain.com, and test.domain.com")
	}

	// Wildcard stuff
	hosts = parseLine("192.168.0.1 *.domain.com mail.domain.com serenity")
	if !hosts.Contains(newHostname("domain.com", net.ParseIP("192.168.0.1"), false, true)) ||
		!hosts.Contains(newHostname("mail.domain.com", net.ParseIP("192.168.0.1"), false, false)) ||
		!hosts.Contains(newHostname("serenity", net.ParseIP("192.168.0.1"), false, false)) ||
		len(hosts) != 3 {
		t.Error("Expected to find *.domain.com, mail.domain.com and serenity.")
	}

	var ip net.IP;

	ip = hosts.FindHost("api.domain.com");
	if !net.ParseIP("192.168.0.1").Equal(ip) {
		t.Error("Can't match wildcard host api.domain.com");
	}

	ip = hosts.FindHost("google.com")
	if ip != nil {
		t.Error("We shouldn't resolve google.com");
	}

	hosts = *newHostlistString(`192.168.0.1 *.domain.com mail.domain.com serenity
				192.168.0.2	api.domain.com`);

	if (!net.ParseIP("192.168.0.2").Equal(hosts.FindHost("api.domain.com"))) {
		t.Error("Failed matching api.domain.com explicitly");
	}
	if (!net.ParseIP("192.168.0.1").Equal(hosts.FindHost("mail.domain.com"))) {
		t.Error("Failed matching api.domain.com explicitly");
	}
	if (!net.ParseIP("192.168.0.1").Equal(hosts.FindHost("wildcard.domain.com"))) {
		t.Error("Failed matching wildcard.domain.com explicitly");
	}
	if (net.ParseIP("192.168.0.1").Equal(hosts.FindHost("sub.wildcard.domain.com"))) {
		t.Error("Failed not matching sub.wildcard.domain.com explicitly");
	}

	// IPv6 (not link-local)
	hosts = parseLine("2a02:7a8:1:250::80:1		rtvslo.si img.rtvslo.si")
	if !hosts.Contains(newHostname("img.rtvslo.si", net.ParseIP("2a02:7a8:1:250::80:1"), true, false)) ||
		len(hosts) != 2 {
		t.Error("Expected to find rtvslo.si ipv6, two hosts")
	}

	// Loopback addresses
	hosts = parseLine("::1 blocked.domain")
	if !hosts.Contains(newHostname("blocked.domain", net.ParseIP("::1"), true, false)) ||
		len(hosts) != 1 {
		t.Error("Expected to find blocked.domain ipv6")
	}

	hosts = parseLine("127.0.0.1 blocked.domain")
	if !hosts.Contains(newHostname("blocked.domain", net.ParseIP("127.0.0.1"), false, false)) ||
		len(hosts) != 1 {
		t.Error("Expected to find blocked.domain ipv4")
	}
}

func TestHostname(t *testing.T) {
	h := newHostname(domain, net.ParseIP(ip), ipv6, wildcard)

	if h.domain != domain {
		t.Errorf("Domain should be %s", domain)
	}
	if !h.ip.Equal(net.ParseIP(ip)) {
		t.Errorf("IP should be %s", ip)
	}
	if h.ipv6 != ipv6 {
		t.Errorf("IPv6 should be %t", ipv6)
	}
	if h.wildcard != wildcard {
		t.Errorf("Wildcard should be %t", wildcard)
	}
}
