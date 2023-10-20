package require

import (
	"regexp"
)

var libRegexp = regexp.MustCompile(`^/lib/(.+).js$`)
