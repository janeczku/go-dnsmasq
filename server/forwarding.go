// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Copyright (c) 2015 Jan Broer
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package server

import (
	"strings"

	"github.com/janeczku/go-dnsmasq/dns"
)

// ServeDNSForward forwards a request to a nameservers and returns the response.
func (s *server) ServeDNSForward(w dns.ResponseWriter, req *dns.Msg) *dns.Msg {
	if s.config.NoRec || len(s.config.Nameservers) == 0 {
		m := new(dns.Msg)
		m.SetReply(req)
		m.SetRcode(req, dns.RcodeServerFailure)
		m.Authoritative = false
		m.RecursionAvailable = false
		if len(s.config.Nameservers) == 0 {
			if s.config.Verbose {
				logf("can not forward, no nameservers defined")
			}
			m.RecursionAvailable = true
		} else {
			m.RecursionAvailable = false
		}

		w.WriteMsg(m)
		return m
	}

	reqOrig := req.Copy()
	name := req.Question[0].Name
	searchFix := false
	searchCname := new(dns.CNAME)

	if dns.CountLabel(name) < 2 || dns.CountLabel(name) < s.config.Ndots {
		if dns.CountLabel(name) < 2 && s.config.SearchDomain != "" {
			searchFix = true
			// append search domain to single-label query name
			nameFqdn := strings.ToLower(appendDomain(name, s.config.SearchDomain))
			searchCname.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 360}
			searchCname.Target = nameFqdn
			// req.Question[0].Name = target
			req.Question[0] = dns.Question{nameFqdn, req.Question[0].Qtype, req.Question[0].Qclass}
		} else {
			if s.config.Verbose {
				logf("can not forward, name too short: `%s'", name)
			}
			m := new(dns.Msg)
			m.SetReply(req)
			m.SetRcode(req, dns.RcodeServerFailure)
			m.Authoritative = false
			m.RecursionAvailable = true
			w.WriteMsg(m)
			return m
		}
	}

	tcp := isTCP(w)

	var (
		r   *dns.Msg
		err error
		try int
	)
	// Use request Id for "random" nameserver selection.
	nsid := int(req.Id) % len(s.config.Nameservers)
Redo:
	switch tcp {
	case false:
		r, _, err = s.dnsUDPclient.Exchange(req, s.config.Nameservers[nsid])
	case true:
		r, _, err = s.dnsTCPclient.Exchange(req, s.config.Nameservers[nsid])
	}
	if err == nil {
		if searchFix {
			// Insert CName resolving hostname to hostname.searchdomain
			if len(r.Answer) > 0 {
				answers := []dns.RR{searchCname}
				for _, rr := range r.Answer {
					answers = append(answers, rr)
				}
				r.Answer = answers
			}
			// Restore original question
			r.Question[0] = reqOrig.Question[0]
		}
		r.Compress = true
		r.Id = req.Id
		w.WriteMsg(r)
		return r
	}
	// Seen an error, this can only mean, "server not reached", try again
	// but only if we have not exausted our nameservers.
	if try < len(s.config.Nameservers) {
		try++
		nsid = (nsid + 1) % len(s.config.Nameservers)
		goto Redo
	}

	logf("failure to forward request %q", err)
	m := new(dns.Msg)
	m.SetReply(reqOrig)
	m.SetRcode(reqOrig, dns.RcodeServerFailure)
	w.WriteMsg(m)
	return m
}

// ServeDNSReverse is the handler for DNS requests for the reverse zone. If nothing is found
// locally the request is forwarded to the forwarder for resolution.
func (s *server) ServeDNSReverse(w dns.ResponseWriter, req *dns.Msg) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(req)
	m.Compress = true
	m.Authoritative = false
	m.RecursionAvailable = true
	if records, err := s.PTRRecords(req.Question[0]); err == nil && len(records) > 0 {
		m.Answer = records
		if err := w.WriteMsg(m); err != nil {
			logf("failure to return reply %q", err)
		}
		return m
	}
	// Always forward if not found locally.
	return s.ServeDNSForward(w, req)
}
