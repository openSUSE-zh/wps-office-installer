// Name: WPS Installer for openSUSE
// Version: 2.0.0
// Description: Install WPS Office onto your openSUSE Box
// Author: Marguerite Su <marguerite@opensuse.org>
// License: GPL-3.0-and-later

package main

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

func getArch() string {
	if strings.HasSuffix(runtime.GOARCH, "64") {
		return "x86_64"
	}
	return "x86"
}

func checkError(e error) {
	if e != nil {
		panic(e)
	}
}

func printDownloadPercent(done chan int64, path string, total int64) {

	stop := false

	for {
		select {
		case <-done:
			stop = true
		default:

			file, err := os.Open(path)
			if err != nil {
				log.Fatal(err)
			}

			fi, err := file.Stat()
			if err != nil {
				log.Fatal(err)
			}

			size := fi.Size()

			if size == 0 {
				size = 1
			}

			percent := float64(size) / float64(total) * 100

			log.Printf("%.0f%%", percent)
		}

		if stop {
			break
		}

		time.Sleep(time.Second)
	}
}

func download(uri, path string) {
	file, err := os.Create(path)
	checkError(err)
	defer file.Close()

	start := time.Now()
	head, err := http.Head(uri)
	checkError(err)
	defer head.Body.Close()
	size, err := strconv.Atoi(head.Header.Get("Content-Length"))
	checkError(err)

	done := make(chan int64)

	go printDownloadPercent(done, path, int64(size))

	resp, err := http.Get(uri)
	checkError(err)
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		panic("File not found: " + uri)
	}

	n, err := io.Copy(file, resp.Body)
	checkError(err)

	done <- n
	elapsed := time.Since(start)
	log.Printf("downloaded "+uri+" to "+path+" in %s", elapsed)
}

func createDir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Println("Creating " + dir)
		err = os.MkdirAll(dir, 0755)
		checkError(err)
	}
}

func unpack(tar, destDir string) {
	log.Println("Unpacking...it'll take some time...")
	createDir(destDir)
	_, err := exec.Command("/usr/bin/tar", "-xf", tar, "--strip-components=1", "-C", destDir).Output()
	checkError(err)
	log.Println("Done!")
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

func absolutePath(relativePath, parentDir string) string {
	if strings.HasPrefix(relativePath, "/") {
		return filepath.Clean(relativePath)
	}
	return filepath.Join(parentDir, relativePath)
}

func parentDir(file string) string {
	absPath, err := filepath.Abs(file)
	checkError(err)

	return path.Dir(absPath)
}

func followSymlink(file string) (string, int) {
	var link string
	dir := parentDir(file)

	link, err := filepath.EvalSymlinks(file)
	checkError(err)

	re := regexp.MustCompile(`\.\.`)
	matches := re.FindStringSubmatch(link)

	return absolutePath(link, dir), len(matches)
}

func getDest(src, dst string) string {
	dstInfo, err := os.Lstat(dst)
	if err != nil {
		return dst
	}

	if dstInfo.IsDir() {
		getDest(src, filepath.Join(dst, filepath.Base(src)))
	}

	return dst
}

func findOrigDir(dst string, num int) string {
	if num == 0 {
		return dst
	}
	var dir string
	for i := 0; i < num; i++ {
		dir = parentDir(dst)
	}
	return dir
}

func findOrigDest(orig, dir string) string {
	return filepath.Join(dir, filepath.Base(orig))
}

func copyFile(src, dst string) {
	si, err := os.Lstat(src)
	checkError(err)

	dst = getDest(src, dst)
	if _, err = os.Lstat(dst); err == nil {
		os.Remove(dst)
	}

	if si.Mode()&os.ModeSymlink != 0 {
		orig, num := followSymlink(src)
		dstParentDir := parentDir(dst)
		origDir := findOrigDir(dstParentDir, num)
		origDest := findOrigDest(orig, origDir)
		if _, err := os.Stat(origDest); os.IsNotExist(err) {
			copyFile(orig, origDest)
		}
		os.Symlink(origDest, absolutePath(dst, dstParentDir))
	} else {
		in, err := os.Open(src)
		checkError(err)
		defer in.Close()

		out, err := os.Create(dst)
		checkError(err)
		defer func() {
			if e := out.Close(); e != nil {
				err = e
			}
		}()

		_, err = io.Copy(out, in)
		checkError(err)

		err = out.Sync()
		checkError(err)

		err = os.Chmod(dst, si.Mode())
		checkError(err)
	}
}

func copyDir(src, dst string) {
	srcParentDir := parentDir(src)
	dstParentDir := parentDir(dst)
	src = absolutePath(src, srcParentDir)
	dst = getDest(src, absolutePath(dst, dstParentDir))

	srcInfo, err := os.Stat(src)
	checkError(err)

	if srcInfo.IsDir() {
		if _, err := os.Lstat(dst); err != nil {
			os.MkdirAll(dst, srcInfo.Mode())
		}
		entries, err := ioutil.ReadDir(src)
		checkError(err)

		for _, entry := range entries {
			srcPath := filepath.Join(src, entry.Name())
			dstPath := filepath.Join(dst, entry.Name())
			if entry.IsDir() {
				copyDir(srcPath, dstPath)
			} else {
				copyFile(srcPath, dstPath)
			}
		}
	} else {
		copyFile(src, dst)
	}
}

func replaceBinPath(p, binPath string) {
	file, err := ioutil.ReadFile(p)
	checkError(err)

	fileInfo, err := os.Lstat(p)
	checkError(err)

	re := regexp.MustCompile("(?m)^gBinPath=.*?\n")
	str := re.ReplaceAllString(string(file), "gBinPath=\""+binPath+"\"\n")

	err = ioutil.WriteFile(p, []byte(str), fileInfo.Mode())
	checkError(err)
}

func findFilesByExt(dir, ext string) []string {
	var res []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, e error) error {
		if filepath.Ext(p) == ext {
			res = append(res, p)
		}
		return nil
	})
	checkError(err)
	return res
}

func main() {
	if os.Getuid() != 0 {
		panic("Must be root to exectuate this program")
	}

	wpsVer := "10.1.0.6634"
	//wpsAlpha := "a21"
	wpsArch := getArch()
	// http://kdl.cc.ksosoft.com/wps-community/download/6634/wps-office_10.1.0.6634_x86_64.tar.xz
	wpsTar := "wps-office_" + wpsVer + "_" + wpsArch + ".tar.xz"
	wpsURL := "http://kdl.cc.ksosoft.com/wps-community/download/6634/" + wpsTar
	wpsTmp := "/tmp/"
	wpsDir := "wps-office_" + wpsVer + "_" + wpsArch
	wpsPrefix := wpsTmp + wpsDir
	wpsDestDir := "/usr/share/wps-office"
	wpsFontDir := "/usr/share/fonts/wps-office"

	if _, err := os.Stat(wpsTmp + wpsTar); os.IsNotExist(err) {
		log.Println("Downloading proprietary binary from WPS (100+ MB)...slow")
		download(wpsURL, wpsTmp+wpsTar)
		log.Println("Done!")
	}

	unpack(wpsTmp+wpsTar, wpsPrefix)
	createDir(wpsDestDir)
	renameWhitespaceInDir(wpsPrefix)
	renameWhitespaceInFiles(wpsPrefix)

	log.Println("Copying files...Ultra slow...")
	copyDir(wpsPrefix+"/office6", wpsDestDir+"/office6")

	// install binaries
	binaries := [3]string{wpsPrefix + "/et",
		wpsPrefix + "/wps",
		wpsPrefix + "/wpp"}

	for _, file := range binaries {
		replaceBinPath(file, wpsDestDir)
		copyFile(file, "/usr/bin/"+filepath.Base(file))
	}

	// install fonts
	createDir(wpsFontDir)
	fonts := findFilesByExt(wpsPrefix+"/fonts", ".TTF")

	for _, font := range fonts {
		copyFile(font, wpsFontDir+"/"+filepath.Base(font))
	}

	copyFile(wpsPrefix+"/fontconfig/40-wps-office.conf", "/usr/share/fontconfig/conf.avail/40-wps-office.conf")

	_, err := exec.Command("/usr/bin/fc-cache", "-f").Output()
	checkError(err)

	// install desktop files
	desktops := findFilesByExt(wpsPrefix+"/resource/applications", ".desktop")

	for _, d := range desktops {
		copyFile(d, "/usr/share/applications/"+filepath.Base(d))
	}

	_, err = exec.Command("/usr/bin/update-desktop-database", "/usr/share/applications", "&>/dev/null").Output()
	checkError(err)

	// install icons
	icons := findFilesByExt(wpsPrefix+"/resource/icons/hicolor", ".png")

	for _, icon := range icons {
		dest := strings.Replace(icon, wpsPrefix+"/resource", "/usr/share", 1)
		copyFile(icon, dest)
	}

	_, err = exec.Command("/usr/bin/gtk-update-icon-cache", "--quiet", "--force", "/usr/share/icons/hicolor").Output()
	checkError(err)

	// install mimetypes
	xmls := findFilesByExt(wpsPrefix+"/resource/mime/packages", ".xml")

	for _, xml := range xmls {
		copyFile(xml, "/usr/share/mime/packages/"+filepath.Base(xml))
	}

	_, err = exec.Command("/usr/bin/update-mime-database", "/usr/share/mime").Output()
	checkError(err)

        // install desktop-directories
	createDir("/usr/share/desktop-directories")
        dirs := findFilesByExt(wpsPrefix+"/resource/desktop-directories", ".directory")
	for _, d := range dirs {
		copyFile(d, "/usr/share/desktop-directories/"+filepath.Base(d))
	}

	// install templates
	createDir("/usr/share/templates")
	createDir("/usr/share/templates/.source")
	templates := findFilesByExt(wpsPrefix+"/resource/templates", ".desktop")
	for _, t := range templates {
		copyFile(t, "/usr/share/templates/"+filepath.Base(t))
	}

	os.RemoveAll(wpsPrefix)

	log.Println("Congratulations! Installation succeed!")
}
