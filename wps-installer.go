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
	"strings"
)

func getArch() string {
	if strings.HasSuffix(runtime.GOARCH, "64") {
		return "x86_64"
	}
	return "x86"
}

func checkError(e error) error {
	if e != nil {
		return e
	}
	return nil
}

func download(uri, path string) {
	file, err := os.Create(path)
	checkError(err)
	defer file.Close()

	resp, err := http.Get(uri)
	checkError(err)
	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	checkError(err)

	log.Println("downloaded " + uri + " to " + path)
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
			os.Rename(p, strings.Replace(p, " ", "_", -1))
		}
	}
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

func followSymlink(file string) string {
	var link string
	dir := parentDir(file)

	link, err := filepath.EvalSymlinks(file)
	checkError(err)

	return absolutePath(link, dir)
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

func copyFile(src, dst string) error {
	si, err := os.Lstat(src)
	checkError(err)

	dst = getDest(src, dst)
	if _, err = os.Lstat(dst); err == nil {
		os.Remove(dst)
	}

	if si.Mode()&os.ModeSymlink != 0 {
		orig := followSymlink(src)
		dstParentDir := parentDir(dst)
		os.Symlink(orig, absolutePath(dst, dstParentDir))
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

	return err
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

	wpsVer := "10.1.0.5707"
	wpsAlpha := "a21"
	wpsArch := getArch()
	wpsTar := "wps-office_" + wpsVer + "~" + wpsAlpha + "_" + wpsArch + ".tar.xz"
	wpsURL := "http://kdl1.cache.wps.com/kodl/download/linux/" + wpsAlpha + "//" + wpsTar
	wpsTmp := "/tmp/"
	wpsDir := "wps-office_" + wpsVer + "_" + wpsArch
	wpsPrefix := wpsTmp + wpsDir
	wpsDestDir := "/usr/share/wps-office"
	wpsFontDir := "/usr/share/fonts/wps-office"

	if _, err := os.Stat(wpsTmp + wpsTar); !os.IsNotExist(err) {
		log.Println("Downloading proprietary binary from WPS (100+ MB)...slow")
		download(wpsURL, wpsTmp+wpsTar)
		log.Println("Done!")
	}

	unpack(wpsTar, wpsPrefix)
	createDir(wpsDestDir)
	renameWhitespaceInDir(wpsPrefix)

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

	os.RemoveAll(wpsPrefix)

	log.Println("Congratulations! Installation succeed!")
}
