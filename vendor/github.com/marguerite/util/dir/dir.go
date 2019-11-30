package dir

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/marguerite/util"
	"github.com/marguerite/util/slice"
)

func pathSeparator() string {
	if runtime.GOOS == "windows" {
		return "\\"
	}
	return "/"
}

// ReadSymlink follows the path of the symlink recursively and finds out the target it finally points to.
func ReadSymlink(path string) (string, error) {
	link, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(link) {
		link = filepath.Join(filepath.Dir(path), link)
	}
	f, err := os.Stat(link)
	if err != nil {
		return link, err
	}
	if f.Mode()&os.ModeSymlink != 0 {
		return ReadSymlink(link)
	}
	return link, nil
}

//ParseWildcard parse **/*, * and {,}
func ParseWildcard(s string) (string, []*regexp.Regexp) {
	r := []*regexp.Regexp{}
	fn := func(p string) *regexp.Regexp {
		p = strings.Replace(p, "**", "*", -1)
		p = strings.Replace(p, "*"+pathSeparator()+"*", "*", -1)
		p = strings.Replace(p, "*", ".*", -1)
		p = strings.Replace(p, "\\", "\\\\", -1)
		return regexp.MustCompile("^" + p + "$")
	}

	fn1 := func(p string) string {
		if strings.HasSuffix(p, pathSeparator()) {
			return p
		}
		return filepath.Dir(p)
	}

	// /usr/lib/xxx.{a, la}
	re := regexp.MustCompile(`([^{]+){([^}]+)}(.*)?`)
	m := re.FindStringSubmatch(s)
	if len(m) != 0 {
		for _, v := range strings.Split(m[2], ",") {
			ns := m[1] + strings.TrimSpace(v)
			if len(m[3]) != 0 {
				ns += m[3]
			}
			r = append(r, fn(ns))
		}
		return fn1(m[1]), r
	}
	if strings.Contains(s, "*") {
		r = append(r, fn(s))
		return fn1(strings.Split(s, "*")[0]), r
	}
	return s, r
}

func ls(d string, kind string) ([]string, error) {
	files := []string{}
	d1, _ := filepath.Abs(d)
	d2, r := ParseWildcard(d1)

	f, err := os.Stat(d2)
	if err != nil {
		return files, err
	}
	if f.Mode().IsRegular() {
		return []string{d2}, nil
	}

	err1 := filepath.Walk(d2, func(path string, info os.FileInfo, err2 error) error {
		if err2 != nil {
			if os.IsPermission(err2) {
				fmt.Println("WARNING: no permission to visit " + path + ", skipped")
				return nil
			}
			return err2
		}

		// filter
		ok := util.MatchMultiRegexps(path, r)
		if !ok {
			return nil
		}

		switch kind {
		case "dir":
			if info.IsDir() {
				files = append(files, path)
			}
			// the symlinks to directories
			if info.Mode()&os.ModeSymlink != 0 {
				link, err3 := ReadSymlink(path)
				if err3 != nil {
					if os.IsNotExist(err) {
						// the symlink points to an non-existent target, ignore
						fmt.Println("WARNING: " + path + " points to an non-existent target " + link)
						return nil
					}
					return err3
				}
				f, _ := os.Stat(link)
				if f.IsDir() {
					files = append(files, path)
				}
			}
		case "symlink":
			if info.Mode()&os.ModeSymlink != 0 {
				files = append(files, path)
			}
		default:
			if info.Mode().IsRegular() {
				files = append(files, path)
			}
			// the symlinks to actual files
			if info.Mode()&os.ModeSymlink != 0 {
				link, err3 := ReadSymlink(path)
				if err3 != nil {
					if os.IsNotExist(err3) {
						// the symlink points to an non-existent target, ignore
						fmt.Println("WARNING: " + path + " points to an non-existent target " + link)
						return nil
					}
					return err3
				}
				f, _ := os.Stat(link)
				if f.Mode().IsRegular() {
					files = append(files, path)
				}
			}
		}
		return nil
	})
	return files, err1
}

// Ls Takes a directory and the kind of file to be listed, returns the list of file and the possible error. Kind supports: dir, symlink, defaults to file.
func Ls(d string, kinds ...string) ([]string, error) {
	if len(kinds) == 0 {
		return ls(d, "")
	}
	f := []string{}
	for _, kind := range kinds {
		i, err := ls(d, kind)
		if err != nil {
			// f is incomplete
			return f, err
		}
		slice.Concat(&f, i)
	}
	return f, nil
}

// MkdirP create directories for path
func MkdirP(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err1 := os.MkdirAll(path, os.ModePerm)
			if err1 != nil {
				return err1
			}
			return nil
		}
		return err
	}
	return nil
}

func parsePattern(re []*regexp.Regexp, pattern interface{}) []*regexp.Regexp {
	switch v := pattern.(type) {
	case *regexp.Regexp:
		re = append(re, v)
	case []*regexp.Regexp:
		for _, i := range v {
			re = append(re, i)
		}
	case string:
		re = append(re, regexp.MustCompile(v))
	case []string:
		for _, i := range v {
			re = append(re, regexp.MustCompile(i))
		}
	default:
		fmt.Println("Unsupported pattern type. Supported: *regexp.Regexp, []*regexp.Regexp, string, []string.")
		os.Exit(1)
	}
	return re
}

// Glob return files in `d` directory that matches `pattern`. can pass `ex` to exclude file from the matches. ex's expanded regex number can be zero (no exclusion), one (test against every expanded regex in pattern), or equals to the number of expanded regex in pattern(one exclude regex refers to one match regex). expanded regex number, eg: [".*\\.yaml","opencc\\/.*"] is one slice param, but the expanded regex number will be two.
func Glob(d string, pattern interface{}, ex ...interface{}) ([]string, error) {
	return glob(d, pattern, Ls, ex...)
}

// fn: used to pass a test function or a real function that involves file operations.
func glob(d string, pattern interface{}, fn func(string, ...string) ([]string, error), ex ...interface{}) ([]string, error) {
	files, err := fn(d)
	if err != nil {
		return []string{}, err
	}

	re := []*regexp.Regexp{}
	re = parsePattern(re, pattern)
	re1 := []*regexp.Regexp{}
	if len(ex) > 0 {
		// ex's type is []interface{}, need to assert to actual type first
		for _, v := range ex {
			re1 = parsePattern(re1, v)
		}
	}

	if len(re) != len(re1) && len(re1) > 1 {
		fmt.Println("We just support exclude regex whose number matches zero, one or the number of regex in pattern.")
		os.Exit(1)
	}

	m := []string{}
	for _, f := range files {
		for i, r := range re {
			if r.MatchString(f) {
				if len(re1) == 0 {
					m = append(m, f)
				} else {
					if len(re1) == 1 {
						if !re1[0].MatchString(f) && len(re1[0].String()) != 0 {
							m = append(m, f)
						}
					} else {
						if !re1[i].MatchString(f) && len(re1[i].String()) != 0 {
							m = append(m, f)
						}
					}
				}

			}
		}
	}
	return m, nil
}
