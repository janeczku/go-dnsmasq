// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package cache

import (
	"time"

	"github.com/miekg/dns"
)

// Hit returns a dns message, expired bool, and key from the cache. The caller must
// decide whether or not to remove the cache entry if expired.
func (c *Cache) Hit(question dns.Question, dnssec, tcp bool, msgid uint16) (m1 *dns.Msg, expired bool, key string) {
	key = Key(question, dnssec, tcp)
	m1, exp, hit := c.Search(key)
	if hit {
		// Cache hit! \o/
		m1.Id = msgid
		m1.Compress = true
		// Even if something ended up with the TC bit *in* the cache, set it to off
		m1.Truncated = false

		// we let the caller decide if the cache entry should be deleted
		expired = time.Since(exp) >= 0
	}
    return
}
