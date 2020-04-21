package fileutils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/marguerite/util"
	"github.com/marguerite/util/dir"
)

//Touch touch a file
func Touch(path string) error {
	_, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			// create containing directory
			d := filepath.Dir(path)
			if d != "." {
				if _, err1 := os.Stat(d); err1 != nil {
					if os.IsNotExist(err1) {
						err2 := dir.MkdirP(d)
						if err2 != nil {
							fmt.Printf("Can not create containing directory %s.\n", d)
							return err2
						}
					} else {
						return err1
					}
				}
			}
			f, err1 := os.Create(path)
			defer f.Close()
			if err1 != nil {
				return err1
			}
		} else {
			return err
		}
	}
	return nil
}

//cp copy a single file to another file or directory
func cp(source, destination, original string) error {
	// source always exists and can be file only
	s, err := ioutil.ReadFile(source)
	if err != nil {
		return err
	}

	// destination can be non-existent target, file or directory.
	di, err := os.Stat(destination)

	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err == nil && di.Mode().IsDir() {
		destination = filepath.Join(destination, filepath.Base(source))
		original = ""
	}

	if os.IsNotExist(err) {
		err = dir.MkdirP(filepath.Dir(destination))
		if err != nil {
			return err
		}
	}

	fi, _ := os.Stat(source)
	err = ioutil.WriteFile(destination, s, fi.Mode())
	if err != nil {
		return err
	}

	if len(original) > 0 {
		err := os.RemoveAll(original)
		if err != nil {
			return err
		}
		err = os.Symlink(destination, original)
		if err != nil {
			return err
		}
	}
	return nil
}

func copy(source string, destination string, re []*regexp.Regexp, fn func(s, d, o string) error) ([]string, error) {
	copyed := []string{}

	// check source status
	srcInfo, err := os.Stat(source)
	if err != nil {
		return copyed, err
	}

	// source is a symlink, copy its original content
	if srcInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
		fmt.Printf("%s is a symlink, copying the original file.\n", source)
		link, err := dir.ReadSymlink(source)
		if err != nil {
			return copyed, err
		}
		tmp, _ := os.Stat(link)
		srcInfo = tmp
		source = link
	}

	// check destination status
	dstInfo, err := os.Stat(destination)
	// destination can be non-existent target
	if err != nil && !os.IsNotExist(err) {
		return copyed, err
	}

	var orig string

	if err == nil && dstInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
		fmt.Printf("%s is a symlink, copy to its original file and symlink back.\n", destination)
		link, err := dir.ReadSymlink(destination)
		if err != nil {
			return copyed, err
		}
		orig = destination
		destination = link
	}

	// copy single file
	if srcInfo.Mode().IsRegular() {
		err := fn(source, destination, orig)
		if err != nil {
			return copyed, err
		}
		return []string{source}, nil
	}
	// copy directory
	if srcInfo.Mode().IsDir() {
		// files can be symlink or actual file
		files, err := dir.Ls(source)
		if err != nil {
			return copyed, err
		}

		for _, f := range files {
			ok := util.MatchMultiRegexps(f, re)
			if !ok {
				continue
			}

			fi, err := os.Stat(f)
			if err != nil {
				fmt.Printf("skipped %s.\n", f)
				continue
			}

			// keep hierarchy
			p, _ := filepath.Rel(source, filepath.Dir(f))
			dest := filepath.Join(destination, p, filepath.Base(f))

			// f is a symlink, copy its original content
			if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
				fmt.Printf("%s is a symlink, copying the original file.\n", f)
				link, err := dir.ReadSymlink(f)
				if err != nil {
					fmt.Printf("skipped %s.\n", f)
					continue
				}
				f = link
			}

			err = fn(f, dest, "")
			if err != nil {
				return copyed, err
			}
			copyed = append(copyed, f)
		}
		return copyed, nil
	}
	return copyed, fmt.Errorf("source %s has unknown filemode %v", source, srcInfo)
}

// Copy like Linux's cp command, copy a file/dirctory to another place.
func Copy(src, dest string) error {
	f, r := dir.ParseWildcard(src)
	_, err := copy(f, dest, r, cp)
	return err
}

//HasPrefixOrSuffix Check if string s has prefix or suffix provided by the variadic slice
// the slice can be []string or [][]string, which means you can group prefixes and suffixes
// >0 means prefix, <0 means suffix, ==0 means no match.
func HasPrefixOrSuffix(s string, seps ...interface{}) int {
	if len(seps) == 0 {
		return 0
	}

	sepKd := reflect.ValueOf(seps[0]).Kind()

	if sepKd == reflect.String {
		// seps is a slice of string, just test prefix or suffix
		for _, sep := range seps {
			sepS := reflect.ValueOf(sep).String()
			if strings.HasPrefix(s, sepS) {
				return 1
			}
			if strings.HasSuffix(s, sepS) {
				return -1
			}
		}
		return 0
	}

	if sepKd == reflect.Array || sepKd == reflect.Slice {
		for _, sep := range seps {
			v := reflect.ValueOf(sep)
			if v.Index(0).Kind() != reflect.String {
				fmt.Println("You must provide a slice of string, or a slice of string slice to check prefix/suffix against the provided string.")
				os.Exit(1)
			}
			for i := 0; i < v.Len(); i++ {
				sepS := v.Index(i).String()
				if strings.HasPrefix(s, sepS) {
					return 1
				}
				if strings.HasSuffix(s, sepS) {
					return -1
				}
			}
		}
	}

	return 0
}
