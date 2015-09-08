// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Copyright (c) 2015 Jan Broer
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package server

import (
	"net"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

type server struct {
	hosts   Hostfile
	config  *Config
	version string

	group        *sync.WaitGroup
	dnsUDPclient *dns.Client // used for forwarding queries
	dnsTCPclient *dns.Client // used for forwarding queries
}

type Hostfile interface {
	FindHosts(name string) ([]net.IP, error)
	FindReverse(name string) (string, error)
}

// New returns a new server.
func New(hostfile Hostfile, config *Config, v string) *server {
	return &server{
		hosts:   hostfile,
		config:  config,
		version: v,

		group:        new(sync.WaitGroup),
		dnsUDPclient: &dns.Client{Net: "udp", ReadTimeout: 2 * config.ReadTimeout, WriteTimeout: 2 * config.ReadTimeout, SingleInflight: true},
		dnsTCPclient: &dns.Client{Net: "tcp", ReadTimeout: 2 * config.ReadTimeout, WriteTimeout: 2 * config.ReadTimeout, SingleInflight: true},
	}
}

// Run is a blocking operation that starts the server listening on the DNS ports.
func (s *server) Run() error {
	mux := dns.NewServeMux()
	mux.Handle(".", s)

	dnsReadyMsg := func(addr, net string) {
		logf("ready for queries on %s://%s - nameservers: %v", net, addr, s.config.Nameservers)
	}

	s.group.Add(1)
	go func() {
		defer s.group.Done()
		if err := dns.ListenAndServe(s.config.DnsAddr, "tcp", mux); err != nil {
			fatalf("%s", err)
		}
	}()
	dnsReadyMsg(s.config.DnsAddr, "tcp")
	s.group.Add(1)
	go func() {
		defer s.group.Done()
		if err := dns.ListenAndServe(s.config.DnsAddr, "udp", mux); err != nil {
			fatalf("%s", err)
		}
	}()
	dnsReadyMsg(s.config.DnsAddr, "udp")

	s.group.Wait()
	return nil
}

// Stop stops a server.
func (s *server) Stop() {
	// TODO(miek)
	//s.group.Add(-2)
}

// ServeDNS is the handler for DNS requests, responsible for parsing DNS request, possibly forwarding
// it to a real dns server and returning a response.
func (s *server) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(req)
	m.Authoritative = false
	m.RecursionAvailable = true
	m.Compress = true
	bufsize := uint16(512)
	dnssec := false
	tcp := false
	local := true

	q := req.Question[0]
	name := strings.ToLower(q.Name)

	/*	if q.Qtype == dns.TypeANY {
		m.Authoritative = false
		m.Rcode = dns.RcodeRefused
		m.RecursionAvailable = false
		m.RecursionDesired = false
		m.Compress = false
		// if write fails don't care
		w.WriteMsg(m)

		return
	}*/

	if o := req.IsEdns0(); o != nil {
		bufsize = o.UDPSize()
		dnssec = o.Do()
	}
	if bufsize < 512 {
		bufsize = 512
	}
	// with TCP we can send 64K
	if tcp = isTCP(w); tcp {
		bufsize = dns.MaxMsgSize - 1
	}

	if s.config.Verbose {
		logf("received DNS Request for %q from %q with type %d", q.Name, w.RemoteAddr(), q.Qtype)
	}

	defer func() {
		if local {
			if m.Rcode == dns.RcodeServerFailure {
				if err := w.WriteMsg(m); err != nil {
					logf("failure to return reply %q", err)
				}
				return
			}

			if tcp {
				if _, overflow := Fit(m, dns.MaxMsgSize, tcp); overflow {
					msgFail := new(dns.Msg)
					s.ServerFailure(msgFail, req)
					w.WriteMsg(msgFail)
					return
				}
			} else {
				Fit(m, int(bufsize), tcp)
			}

			if err := w.WriteMsg(m); err != nil {
				logf("failure to return reply %q", err)
			}
		}
	}()

	// Check hosts records before forwarding the query
	if q.Qtype == dns.TypeA || q.Qtype == dns.TypeAAAA || q.Qtype == dns.TypeANY {
		records, err := s.AddressRecords(q, name)
		if err != nil {
			logf("error from backend: %s", err)
		}
		if len(records) > 0 {
			m.Answer = append(m.Answer, records...)
			return
		}
	}

	if q.Qtype == dns.TypePTR && strings.HasSuffix(name, ".in-addr.arpa.") || strings.HasSuffix(name, ".ip6.arpa.") {
		local = false
		s.ServeDNSReverse(w, req)
		return
	}

	if q.Qclass == dns.ClassCHAOS {
		m.Authoritative = true
		if q.Qtype == dns.TypeTXT {
			switch name {
			case "authors.bind.":
				hdr := dns.RR_Header{Name: q.Name, Rrtype: dns.TypeTXT, Class: dns.ClassCHAOS, Ttl: 0}
				authors := []string{"Erik St. Martin", "Brian Ketelsen", "Miek Gieben", "Michael Crosby", "Jan Broer"}
				for _, a := range authors {
					m.Answer = append(m.Answer, &dns.TXT{Hdr: hdr, Txt: []string{a}})
				}
				for j := 0; j < len(authors)*(int(dns.Id())%4+1); j++ {
					q := int(dns.Id()) % len(authors)
					p := int(dns.Id()) % len(authors)
					if q == p {
						p = (p + 1) % len(authors)
					}
					m.Answer[q], m.Answer[p] = m.Answer[p], m.Answer[q]
				}
				return
			case "version.bind.":
				fallthrough
			case "version.server.":
				hdr := dns.RR_Header{Name: q.Name, Rrtype: dns.TypeTXT, Class: dns.ClassCHAOS, Ttl: 0}
				m.Answer = []dns.RR{&dns.TXT{Hdr: hdr, Txt: []string{s.version}}}
				return
			case "hostname.bind.":
				fallthrough
			case "id.server.":
				// TODO(miek): machine name to return
				hdr := dns.RR_Header{Name: q.Name, Rrtype: dns.TypeTXT, Class: dns.ClassCHAOS, Ttl: 0}
				m.Answer = []dns.RR{&dns.TXT{Hdr: hdr, Txt: []string{"localhost"}}}
				return
			}
		}
		// still here, fail
		m.SetReply(req)
		m.SetRcode(req, dns.RcodeServerFailure)
		return
	}

	// Forward queries matching stub zones
	for zone, ns := range *s.config.Stub {
		if strings.HasSuffix(name, zone) {
			local = false
			s.ServeDNSStubForward(w, req, ns)
			return
		}
	}

	// Forward all other queries
	local = false
	s.ServeDNSForward(w, req)

}

func (s *server) AddressRecords(q dns.Question, name string) (records []dns.RR, err error) {
	results, err := s.hosts.FindHosts(name)
	if err != nil {
		return nil, err
	}

	for _, ip := range results {
		switch {
		case ip.To4() != nil && (q.Qtype == dns.TypeA || q.Qtype == dns.TypeANY):
			r := new(dns.A)
			r.Hdr = dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA,
				Class: dns.ClassINET, Ttl: s.config.HostsTtl}
			r.A = ip.To4()
			records = append(records, r)
		case ip.To4() == nil && (q.Qtype == dns.TypeAAAA || q.Qtype == dns.TypeANY):
			r := new(dns.AAAA)
			r.Hdr = dns.RR_Header{Name: q.Name, Rrtype: dns.TypeAAAA,
				Class: dns.ClassINET, Ttl: s.config.HostsTtl}
			r.AAAA = ip.To16()
			records = append(records, r)
		}
	}
	return records, nil
}

func (s *server) PTRRecords(q dns.Question) (records []dns.RR, err error) {
	name := strings.ToLower(q.Name)
	result, err := s.hosts.FindReverse(name)
	if err != nil {
		return nil, err
	}
	if result != "" {
		r := new(dns.PTR)
		r.Hdr = dns.RR_Header{Name: q.Name, Rrtype: dns.TypePTR,
			Class: dns.ClassINET, Ttl: s.config.HostsTtl}
		r.Ptr = result
		records = append(records, r)
	}
	return records, nil
}

func (s *server) ServerFailure(m, req *dns.Msg) {
	m.SetRcode(req, dns.RcodeServerFailure)
}

func (s *server) RoundRobin(rrs []dns.RR) {
	if !s.config.RoundRobin {
		return
	}
	// If we have more than 1 CNAME don't touch the packet, because some stub resolver (=glibc)
	// can't deal with the returned packet if the CNAMEs need to be accesses in the reverse order.
	cname := 0
	for _, r := range rrs {
		if r.Header().Rrtype == dns.TypeCNAME {
			cname++
			if cname > 1 {
				return
			}
		}
	}

	switch l := len(rrs); l {
	case 2:
		if dns.Id()%2 == 0 {
			rrs[0], rrs[1] = rrs[1], rrs[0]
		}
	default:
		for j := 0; j < l*(int(dns.Id())%4+1); j++ {
			q := int(dns.Id()) % l
			p := int(dns.Id()) % l
			if q == p {
				p = (p + 1) % l
			}
			rrs[q], rrs[p] = rrs[p], rrs[q]
		}
	}

}

// isTCP returns true if the client is connecting over TCP.
func isTCP(w dns.ResponseWriter) bool {
	_, ok := w.RemoteAddr().(*net.TCPAddr)
	return ok
}
