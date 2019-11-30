package util

import (
	"regexp"
)

//MatchMultiRegexps match string against multiple *re.Regexp
func MatchMultiRegexps(f string, regexps []*regexp.Regexp) bool {
	if len(regexps) == 0 {
		return true
	}
	for _, re := range regexps {
		if re.MatchString(f) {
			return true
		}
	}
	return false
}
