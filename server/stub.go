// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package server

import (
	log "github.com/Sirupsen/logrus"
	"github.com/miekg/dns"
)

// ServeDNSStubForward forwards a request to a nameservers and returns the response.
func (s *server) ServeDNSStubForward(w dns.ResponseWriter, req *dns.Msg, ns []string) *dns.Msg {
	StatsStubForwardCount.Inc(1)

	tcp := isTCP(w)

	var (
		r   *dns.Msg
		err error
		try int
	)

	// Use request Id for "random" nameserver selection.
	nsid := int(req.Id) % len(ns)
Redo:
	switch tcp {
	case false:
		r, _, err = s.dnsUDPclient.Exchange(req, ns[nsid])
	case true:
		r, _, err = s.dnsTCPclient.Exchange(req, ns[nsid])
	}
	if err == nil {
		r.Compress = true
		r.Id = req.Id
		w.WriteMsg(r)
		return r
	}
	// Seen an error, this can only mean, "server not reached", try again
	// but only if we have not exausted our nameservers.
	if try < len(ns) {
		try++
		nsid = (nsid + 1) % len(ns)
		goto Redo
	}

	log.Errorf("Failure forwarding stub request %q", err)
	m := new(dns.Msg)
	m.SetReply(req)
	m.SetRcode(req, dns.RcodeServerFailure)
	w.WriteMsg(m)
	return m
}
