package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func checkError(e error) {
	if e != nil {
		panic(e)
	}
}

func findBinaryData(dir string) []string {
	var data []string
	re := regexp.MustCompile(`\.so`)
	err := filepath.Walk(dir, func(p string, i os.FileInfo, e error) error {
		pInfo, err := os.Stat(p)
		checkError(err)
		// skip zero-bit file
		if pInfo.Size() == 0 {
			return nil
		}
		// skip non-execuatable file
		if !strings.Contains(pInfo.Mode().String(), "x") {
			return nil
		}

		if !i.IsDir() {
			if !(pInfo.Mode()&os.ModeSymlink != 0) {
				f, err := os.Open(p)
				checkError(err)
				defer f.Close()

				buffer := make([]byte, 512)
				_, err = f.Read(buffer)
				checkError(err)

				f.Seek(0, 0)

				contentType := http.DetectContentType(buffer)
				if contentType == "application/octet-stream" {
					ext := filepath.Ext(p)
					if ext == "" || re.MatchString(p) {
						data = append(data, p)
					}
				}
			}
		}
		return nil
	})
	checkError(err)
	return data
}

func contains(item string, arr []string) bool {
	re := regexp.MustCompile(regexp.QuoteMeta(filepath.Base(item)))
	for _, i := range arr {
		if re.MatchString(filepath.Base(i)) {
			return true
		}
	}
	return false
}

func parseDepend(str string, wpsDir string, selfBinaries []string) string {
	re := regexp.MustCompile(`[[:space:]](.*)[[:space:]]=>[[:space:]]([^\(]+)`)
	if strings.Contains(str, "=>") {
		matches := re.FindStringSubmatch(str)
		if !strings.Contains(matches[2], wpsDir) {
			if !contains(matches[1], selfBinaries) {
				return matches[1]
			}
		}
	}
	return ""
}

func findRawDepends(file string, depends []string, wpsDir string, selfBinaries []string) []string {
	out, err := exec.Command("ldd", file).Output()
	checkError(err)

	strs := strings.Split(string(out), "\n")

	for _, l := range strs {
		dep := parseDepend(l, wpsDir, selfBinaries)
		if dep != "" {
			if !contains(dep, depends) {
				depends = append(depends, dep)
			}
		}
	}
	return depends
}

func findDepends(files []string) []string {
	var depends []string
	for _, f := range files {
		cmd, err := exec.Command("/usr/bin/zypper", "se", "-f", f).Output()
		if err != nil {
			re := regexp.MustCompile(`(.*?)(\d+)?\.so\.(\d+)`)
			matches := re.FindStringSubmatch(f)
			var res string
			if len(matches) > 4 {
				res = strings.Replace(f, ".so.", "-", -1)
				res = strings.Replace(res, ".", "_", -1)
			} else {
				res = strings.Replace(f, ".so.", "", -1)
			}
			log.Println(f + " probably resolves to " + res + ", but we couldn't be sure since that package isn't installed. do your own research!")
			continue
		}

		out := strings.Split(string(cmd), "|")
		d := strings.Replace(out[len(out)-3], " ", "", -1)
		if !contains(d, depends) {
			depends = append(depends, d)
		}
	}
	return depends
}

func writeFile(files []string, dest string) {
	if _, err := os.Stat(dest); err == nil {
		os.Remove(dest)
	}

	file, err := os.Create(dest)
	checkError(err)
	defer file.Close()

	for _, f := range files {
		_, err = file.WriteString("Requires: " + f + "\n")
		checkError(err)
	}

	err = file.Sync()
	checkError(err)
}

func main() {
	var wpsDir string
	flag.StringVar(&wpsDir, "wpsdir", "", "wps office directory")
	flag.Parse()

	if wpsDir == "" {
		panic("You must specify the unpacked wps office dir with -wpsdir")
	}

	log.Println("wpsDir: " + wpsDir)

	office6Dir := wpsDir + "/opt/kingsoft/wps-office/office6"

	log.Println("Finding all the bianry data from " + office6Dir)

	binaries := findBinaryData(office6Dir)

	var rawDepends []string
	for _, b := range binaries {
		rawDepends = findRawDepends(b, rawDepends, wpsDir, binaries)
	}

	log.Println("Resolving dependencies to package names...")
	depends := findDepends(rawDepends)

	writeFile(depends, "depends.txt")
}
