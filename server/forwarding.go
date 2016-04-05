// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Copyright (c) 2015 Jan Broer
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package server

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/miekg/dns"
)

// ServeDNSForward resolves a query by forwarding to a recursive nameserver
func (s *server) ServeDNSForward(w dns.ResponseWriter, req *dns.Msg) *dns.Msg {
	name := req.Question[0].Name
	nameDots := dns.CountLabel(name)-1
	refuse := false

	switch {
	case s.config.NoRec:
		log.Debugf("Refused query '%s', recursion disabled", name)
		refuse = true
	case len(s.config.Nameservers) == 0:
		log.Debugf("Refused query '%s', no nameservers configured", name)
		refuse = true
	case nameDots < s.config.FwdNdots && !s.config.AppendDomain:
		log.Debugf("Refused query '%s', name too short", name)
		refuse = true
	}

	if refuse {
		m := new(dns.Msg)
		m.SetRcode(req, dns.RcodeRefused)
		w.WriteMsg(m)
		return m
	}

	StatsForwardCount.Inc(1)

	var didAbsolute bool
	var didSearch bool
	var res1, res2 *dns.Msg
	var err1, err2 error

	tcp := isTCP(w)

	// If there are enough dots in the name, let's first give it a
	// try as absolute name
	if nameDots >= s.config.Ndots {
		if nameDots >= s.config.FwdNdots {
			log.Debugf("Doing initial absolute query for qname '%s'", name)
			res1, err1 = s.forwardQuery(req, tcp)
			if err1 != nil {
				log.Errorf("Error forwarding absolute query for qname '%s': %q", name, err1)
			}

			if err1 == nil && res1.Rcode == dns.RcodeSuccess {
				log.Debugf("Sent reply: qname '%s', rcode %s",
					name, dns.RcodeToString[res1.Rcode])
				res1.Compress = true
				res1.Id = req.Id
				w.WriteMsg(res1)
				return res1
			}
			didAbsolute = true
		} else {
			log.Debugf("Not forwarding initial query, name too short: '%s'", name)
		}
	}

	// We do at least one level of search if AppendDomain is set
	// and forwarding did not previously fail
	if err1 == nil && s.config.AppendDomain {
		log.Debugf("Doing search query for qname '%s'", name)
		res2, err2 = s.forwardSearch(req, tcp)
		if err2 != nil {
			log.Errorf("Error forwarding search query for qname '%s': %q", name, err2)
		}

		if err2 == nil && res2.Rcode == dns.RcodeSuccess {
			log.Debugf("Sent reply: qname '%s', rcode %s",
				name, dns.RcodeToString[res2.Rcode])
			res2.Compress = true
			res2.Id = req.Id
			w.WriteMsg(res2)
			return res2
		}
		didSearch = true
	}

	// If the query has not already been tried as is then try it
	// if there are enough dots in the name and searching did not
	// previously fail
	if err2 == nil && !didAbsolute {
		if nameDots >= s.config.FwdNdots {
			log.Debugf("Doing absolute query for qname '%s'", name)
			res1, err1 = s.forwardQuery(req, tcp)
			if err1 != nil {
				log.Errorf("Error forwarding absolute query for qname '%s': %q", name, err1)
			}

			if err1 == nil && res1.Rcode == dns.RcodeSuccess {
				log.Debugf("Sent reply: qname '%s', rcode %s",
					name, dns.RcodeToString[res1.Rcode])
				res1.Compress = true
				res1.Id = req.Id
				w.WriteMsg(res1)
				return res1
			}
			didAbsolute = true
		} else {
			log.Debugf("Not forwarding query, name too short: `%s'", name)
		}
	}

	// If we got here, we didn't get a positive result for the query.
	// If we did an initial absolute query, return that query's result.
	// else return a no-data response with the rcode from the last search we did.
	if didAbsolute && err1 == nil {
		log.Debugf("Sent reply: qname '%s', rcode %s",
					name, dns.RcodeToString[res1.Rcode])
		res1.Compress = true
		res1.Id = req.Id
		w.WriteMsg(res1)
		return res1
	}

	if didSearch && err2 == nil {
		log.Debugf("Sent reply: qname '%s', rcode %s", name, dns.RcodeToString[res2.Rcode])
		m := new(dns.Msg)
		m.SetRcode(req, res2.Rcode)
		w.WriteMsg(m)
		return m
	}

	// If we got here, we encountered an error while forwarding (which we already logged)
	log.Debugf("Sent reply: qname '%s', rcode SRVFAIL", name)
	m := new(dns.Msg)
	m.SetRcode(req, dns.RcodeServerFailure)
	w.WriteMsg(m)
	return m
}

// forwardSearch resolves a query by suffixing with search paths
func (s *server) forwardSearch(req *dns.Msg, tcp bool) (*dns.Msg, error) {
	var r *dns.Msg
	var nodata *dns.Msg // stores the copy of a NODATA reply
	var searchName string // stores the current name suffixed with search domain
	var err error
	var didSearch bool
	name := req.Question[0].Name // original qname
	reqCopy := req.Copy()

	for _, domain := range s.config.SearchDomains {
		if strings.HasSuffix(name, domain) {
			continue
		}

		searchName = strings.ToLower(appendDomain(name, domain))
		reqCopy.Question[0] = dns.Question{searchName, reqCopy.Question[0].Qtype, reqCopy.Question[0].Qclass}
		didSearch = true
		r, err = s.forwardQuery(reqCopy, tcp)
		if err != nil {
			// No server currently available, give up
			break
		}

		switch r.Rcode {
		case dns.RcodeSuccess:
			// In case of NO_DATA keep searching, otherwise a wildcard entry 
			// could keep us from finding the answer higher in the search list
			if len(r.Answer) == 0 && !r.MsgHdr.Truncated {
				nodata = r.Copy()
				continue
			}
		case dns.RcodeNameError:
			fallthrough
		case dns.RcodeServerFailure:
			// try next search element if any
			continue
		}
		// anything else implies that we are done searching
		break
	}

	if !didSearch {
		m := new(dns.Msg)
		m.SetRcode(req, dns.RcodeNameError)
		return m, nil
	}

	if err == nil {
		if r.Rcode == dns.RcodeSuccess {
			if len(r.Answer) > 0 {
				cname := new(dns.CNAME)
				cname.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 360}
				cname.Target = searchName
				answers := []dns.RR{cname}
				for _, rr := range r.Answer {
					answers = append(answers, rr)
				}
				r.Answer = answers
			}
		// If we ever got a NODATA return this instead of a negative result
		} else if nodata != nil {
			r = nodata
		}
		// Restore original question
		r.Question[0] = req.Question[0]
	}

	if err != nil && nodata != nil {
		r = nodata
		r.Question[0] = req.Question[0]
		err = nil
	}

	return r, err
}

// forwardQuery sends the query to nameservers retrying once on error
func (s *server) forwardQuery(req *dns.Msg, tcp bool) (*dns.Msg, error) {
	var nservers []string // Nameservers to use for this query
	var nsIdx int
	var r *dns.Msg
	var err error

	nservers = s.config.Nameservers

	// Check whether the name matches a stub zone
	for zone, srv := range *s.config.Stub {
		if strings.HasSuffix(req.Question[0].Name, zone) {
			nservers = srv
			StatsStubForwardCount.Inc(1)
			break
		}
	}

	for try := 1; try <= 2; try++ {
		log.Debugf("Sending query: ns '%s', qname '%s'",
			nservers[nsIdx], req.Question[0].Name)

		switch tcp {
		case false:
			r, _, err = s.dnsUDPclient.Exchange(req, nservers[nsIdx])
		case true:
			r, _, err = s.dnsTCPclient.Exchange(req, nservers[nsIdx])
		}

		if err == nil {
			log.Debugf("Got reply: ns '%s', qname '%s', rcode %s",
				nservers[nsIdx],req.Question[0].Name, dns.RcodeToString[r.Rcode])
			switch r.Rcode {
			// SUCCESS
			case dns.RcodeSuccess:
				fallthrough
			case dns.RcodeNameError:
				fallthrough
			// NO RECOVERY
			case dns.RcodeFormatError:
				fallthrough
			case dns.RcodeRefused:
				fallthrough
			case dns.RcodeNotImplemented:
				return r, err
			}
		}

		if err != nil {
			log.Debugf("Query failed: ns '%s', qname '%s', error: %s",
				nservers[nsIdx], req.Question[0].Name, err.Error())
		}

		// Continue with next available server
		if len(nservers)-1 > nsIdx {
			nsIdx++
		} else {
			nsIdx = 0
		}
	}

	return r, err
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
			log.Errorf("Failed to send reply: %q", err)
		}
		return m
	}
	// Always forward if not found locally.
	return s.ServeDNSForward(w, req)
}
