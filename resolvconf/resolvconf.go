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
	"os"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

const RESOLVCONF_COMMENT_ADD = "# added by go-dnsmasq"
const RESOLVCONF_COMMENT_OUT = "# disabled by go-dnsmasq #"
const RESOLVCONF_PATH = "/etc/resolv.conf"

var resolvConfPattern = regexp.MustCompile("(?m:^.*" + regexp.QuoteMeta(RESOLVCONF_COMMENT_ADD) + ")(?:$|\n)")

func StoreAddress(address string) error {
	log.Infof("Setting host nameserver to %s", address)
	resolveConfEntry := fmt.Sprintf("nameserver %s %s\n", address, RESOLVCONF_COMMENT_ADD)
	return updateResolvConf(resolveConfEntry, RESOLVCONF_PATH)
}

func Clean() {
	updateResolvConf("", RESOLVCONF_PATH)
}

func updateResolvConf(insert, path string) error {
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
		switch insert {
		case "":
			// uncomment lines we commented
			if strings.Contains(line, RESOLVCONF_COMMENT_OUT) {
				line = strings.Replace(line, RESOLVCONF_COMMENT_OUT, "", -1)
				line = strings.TrimLeft(line, " ")
			}
		default:
			// comment out active nameservers only
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "nameserver") {
				line = fmt.Sprintf("%s %s", RESOLVCONF_COMMENT_OUT, line)
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
