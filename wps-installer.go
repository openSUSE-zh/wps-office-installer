// Name: WPS Installer for openSUSE
// Version: 2.0.0
// Description: Install WPS Office onto your openSUSE Box
// Author: Marguerite Su <marguerite@opensuse.org>
// License: GPL-3.0-and-later

package main

import (
  "io"
  "path/filepath"
  "net/http"
  "log"
  "os"
  "os/exec"
  "runtime"
  "strings"
)

func get_arch() string {
  if strings.HasSuffix(runtime.GOARCH, "64") {
    return "x86_64"
  } else {
    return "x86"
  }
}

func check_error(e error) {
  if e != nil { panic(e) }
}

func download(uri, path string) {
  file, err := os.Create(path)
  check_error(err)
  defer file.Close()

  resp, err := http.Get(uri)
  check_error(err)
  defer resp.Body.Close()

  _, err = io.Copy(file, resp.Body)
  check_error(err)

  log.Println("downloaded " + uri + " to " + path)
}

func createdir(dir string) {
  if _, err := os.Stat(dir); os.IsNotExist(err) {
    log.Println("Creating " + dir)
    err = os.MkdirAll(dir, 0755)
    check_error(err)
  }
}

func unpack(tar, destdir string) {
  log.Println("Unpacking...it'll take some time...")
  createdir(destdir)
  _, err := exec.Command("/usr/bin/tar", "-xf", tar, "--strip-components=1", "-C", destdir).Output()
  check_error(err)
  log.Println("Done!")
}

func find_all_dirs(root string) ([]string, error) {
    var dirs []string
    err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if info.IsDir() {
            dirs = append(dirs, path)
        }
        return nil
    })
    return dirs, err
}

func rename_whitespace_in_dir(dir string) {
  dirs, err := find_all_dirs(dir)
  check_error(err)
  log.Println(dirs)
}

func main() {
  if os.Getuid() != 0 {
    panic("Must be root to exectuate this program")
  }

  wps_ver := "10.1.0.5707"
  //wps_alpha := "a21"
  wps_arch := get_arch()
  //wps_tar := "wps-office_" + wps_ver + "~" + wps_alpha + "_" + wps_arch + ".tar.xz"
  //wps_url := "http://kdl1.cache.wps.com/kodl/download/linux/" + wps_alpha + "//" + wps_tar
  //wps_tmp := "/tmp/"
  wps_dir := "wps-office_" + wps_ver + "_" + wps_arch
  //wps_destdir := "/usr/share/wps-office"
  //wps_fontdir := "/usr/share/fonts/wps-office"

  //if _, err := os.Stat(wps_tar); !os.IsNotExist(err) {
    log.Println("Downloading proprietary binary from WPS (100+ MB)...slow")
  //  download(wps_url, wps_tmp + wps_tar)
    log.Println("Done!")
  //}

  //unpack(wps_tar, wps_dir)//wps_tmp + wps_dir)
  //createdir(wps_destdir)
  rename_whitespace_in_dir(wps_dir)//wps_tmp + wps_dir)
}
