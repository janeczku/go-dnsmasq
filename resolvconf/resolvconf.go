// Forked and modified from https://github.com/gliderlabs/resolvable/resolver/resolvconf.go
//
// Copyright (c) 2015 Matthew Good
// Copyright (c) 2015 Jan Broer
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package resolvconf

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
)

const RESOLVCONF_COMMENT = "# added by go-dnsmasq"
const RESOLVCONF_PATH = "/etc/resolv.conf"

var resolvConfPattern = regexp.MustCompile("(?m:^.*" + regexp.QuoteMeta(RESOLVCONF_COMMENT) + ")(?:$|\n)")

func StoreAddress(address string) error {
	resolveConfEntry := fmt.Sprintf("nameserver %s %s\n", address, RESOLVCONF_COMMENT)
	return updateResolvConf(resolveConfEntry, RESOLVCONF_PATH)
}

func Clean() {
	updateResolvConf("", RESOLVCONF_PATH)
}

func updateResolvConf(insert, path string) error {
	log.Debugf("Configuring nameservers in /etc/resolv.conf")

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	orig, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	orig = resolvConfPattern.ReplaceAllLiteral(orig, []byte{})

	if _, err = f.Seek(0, os.SEEK_SET); err != nil {
		return err
	}

	if _, err = f.WriteString(insert); err != nil {
		return err
	}

	lines := strings.SplitAfter(string(orig), "\n")
	for _, line := range lines {
		// if file ends in a newline, skip empty string from splitting
		if line == "" {
			continue
		}

		// only comment out name servers
		if strings.Contains(strings.ToLower(line), "nameserver") {
			if insert == "" {
				line = strings.TrimLeft(line, "# ")
			} else {
				line = "# " + line
			}
		}

		if _, err = f.WriteString(line); err != nil {
			return err
		}
	}

	// contents may have been shortened, so truncate where we are
	pos, err := f.Seek(0, os.SEEK_CUR)
	if err != nil {
		return err
	}
	return f.Truncate(pos)
}
