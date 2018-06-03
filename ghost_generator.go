package main

import (
	"flag"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func checkError(e error) {
	if e != nil {
		panic(e)
	}
}

type byLength []string

func (s byLength) Len() int {
	return len(s)
}

func (s byLength) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s byLength) Less(i, j int) bool {
	return len(s[i]) < len(s[j])
}

func getAllSubdir(dir string) []string {
	var dirs []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if info.IsDir() {
			dirs = append(dirs, p)
		}
		return nil
	})
	checkError(err)
	sort.Sort(sort.Reverse(byLength(dirs)))
	return dirs
}

func renameWhitespaceInDir(dir string) {
	dirs := getAllSubdir(dir)
	re := regexp.MustCompile(`[[:space:]]`)

	for _, p := range dirs {
		if re.MatchString(p) {
			os.Rename(p, path.Dir(p)+"/"+strings.Replace(filepath.Base(p), " ", "_", -1))
		}
	}
}

func renameWhitespaceInFiles(dir string) {
	re := regexp.MustCompile(`[[:space:]]`)
	err := filepath.Walk(dir, func(p string, i os.FileInfo, e error) error {
		if !i.IsDir() && re.MatchString(p) {
			os.Rename(p, strings.Replace(p, " ", "_", -1))
		}
		return nil
	})
	checkError(err)
}

func getDirsAndFiles(dir string) ([]string, []string) {
	var dirs []string
	var files []string
	err := filepath.Walk(dir, func(p string, i os.FileInfo, e error) error {
		if i.IsDir() {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
		return nil
	})
	checkError(err)
	return dirs, files
}

func substitute(files []string, orig string, dest string) []string {
	var res []string
	for _, f := range files {
		res = append(res, strings.Replace(f, orig, dest, -1))
	}
	return res
}

func writeFile(files []string, dest string) {
	if _, err := os.Stat(dest); err == nil {
		os.Remove(dest)
	}

	file, err := os.Create(dest)
	checkError(err)
	defer file.Close()

	for _, f := range files {
		_, err = file.WriteString(f + "\n")
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

	office6Dir := wpsDir + "/office6"
	fontsDir := wpsDir + "/fonts"
	renameWhitespaceInDir(office6Dir)
	renameWhitespaceInFiles(office6Dir)
	officeDirs, officeFiles := getDirsAndFiles(office6Dir)
	_, fontFiles := getDirsAndFiles(fontsDir)

	ghostOfficeDirs := substitute(officeDirs, wpsDir, "%dir %{_datadir}/wps-office")
	ghostOfficeFiles := substitute(officeFiles, wpsDir, "%ghost %{_datadir}/wps-office")
	ghostFontFiles := substitute(fontFiles, wpsDir, "%ghost %{_datadir}/fonts/wps-office")
	officeDirs = substitute(officeDirs, wpsDir, "./usr/share/wps-office")

	for _, f := range ghostFontFiles {
		ghostOfficeFiles = append(ghostOfficeFiles, f)
	}

	for _, f := range ghostOfficeDirs {
		ghostOfficeFiles = append(ghostOfficeFiles, f)
	}

	dirs := [3]string{"%dir %{_datadir}/wps-office", "%dir %{_datadir}/wps-office/office6", "%dir %{_datadir}/fonts/wps-office"}

	for _, f := range dirs {
		ghostOfficeFiles = append(ghostOfficeFiles, f)
	}

	officeDirs = append(officeDirs, "./usr/share/fonts/wps-office")

	writeFile(ghostOfficeFiles, "./ghostfiles.txt")
	writeFile(officeDirs, "./wps-dir.txt")
}
