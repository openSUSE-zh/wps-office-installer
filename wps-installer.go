// Name: WPS Installer for openSUSE
// Version: 2.0.0
// Description: Install WPS Office onto your openSUSE Box
// Author: Marguerite Su <marguerite@opensuse.org>
// License: GPL-3.0-and-later

package main

import (
  "io"
  "io/ioutil"
  "path"
  "path/filepath"
  "net/http"
  "log"
  "os"
  "os/exec"
  "regexp"
  "runtime"
  "strings"
  "sort"
)

func get_arch() string {
  if strings.HasSuffix(runtime.GOARCH, "64") {
    return "x86_64"
  } else {
    return "x86"
  }
}

func check_error(e error) error {
  if e != nil { return e }
  return nil
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

func get_all_subdir(dir string) []string {
  var dirs []string
  err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
    if info.IsDir() {
      dirs = append(dirs, path)
    }
    return nil
  })
  check_error(err)
  sort.Sort(sort.Reverse(byLength(dirs)))
  return dirs
}

func rename_whitespace_in_dir(dir string) {
  dirs := get_all_subdir(dir)
  re := regexp.MustCompile(`[[:space:]]`)

  for _, path := range dirs {
    if re.MatchString(path) {
      os.Rename(path, strings.Replace(path, " ", "_", -1))
    }
  }
}

func absolute_path(relative_path, parentdir string) string {
  if strings.HasPrefix(relative_path, "/") {
    return filepath.Clean(relative_path)
  } else {
    return filepath.Join(parentdir, relative_path)
  }
}

func parentdir(file string) string {
  abspath, err := filepath.Abs(file)
  check_error(err)

  return path.Dir(abspath)
}

func follow_symlink(file string) string {
  var link string
  dir := parentdir(file)

  link, err := filepath.EvalSymlinks(file)
  check_error(err)

  return absolute_path(link, dir)
}

func get_dest(src, dst string) string {
  dst_info, err := os.Lstat(dst)
  if err != nil { return dst }

  if dst_info.IsDir() {
    get_dest(src, filepath.Join(dst, filepath.Base(src)))
  } else {
    return dst
  }

  return dst
}

func copyfile(src, dst string) error {
  si, err := os.Lstat(src)
  check_error(err)

  dst = get_dest(src, dst)
  if _, err = os.Lstat(dst); err == nil {
    os.Remove(dst)
  }

  if si.Mode()&os.ModeSymlink != 0 {
    orig := follow_symlink(src)
    dst_parentdir := parentdir(dst)
    os.Symlink(orig, absolute_path(dst, dst_parentdir))
  } else {
    in, err := os.Open(src)
    check_error(err)
    defer in.Close()

    out, err := os.Create(dst)
    check_error(err)
    defer func() {
      if e := out.Close(); e != nil {
        err = e
      }
    }()

    _, err = io.Copy(out, in)
    check_error(err)

    err = out.Sync()
    check_error(err)

    err = os.Chmod(dst, si.Mode())
    check_error(err)
  }

  return err
}

func copydir(src, dst string) {
  src_parentdir := parentdir(src)
  dst_parentdir := parentdir(dst)
  src = absolute_path(src, src_parentdir)
  dst = get_dest(src, absolute_path(dst, dst_parentdir))

  src_info, err := os.Stat(src)
  check_error(err)

  if src_info.IsDir() {
    if _, err := os.Lstat(dst); err != nil {
      os.MkdirAll(dst, src_info.Mode())
    }
    entries, err := ioutil.ReadDir(src)
    check_error(err)

    for _, entry := range entries {
      srcpath := filepath.Join(src, entry.Name())
      dstpath := filepath.Join(dst, entry.Name())
      if entry.IsDir() {
	copydir(srcpath, dstpath)
      } else {
        copyfile(srcpath, dstpath)
      }
    }
  } else {
    copyfile(src, dst)
  }
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
  wps_destdir := "/usr/share/wps-office"
  //wps_fontdir := "/usr/share/fonts/wps-office"

  //if _, err := os.Stat(wps_tar); !os.IsNotExist(err) {
    log.Println("Downloading proprietary binary from WPS (100+ MB)...slow")
  //  download(wps_url, wps_tmp + wps_tar)
    log.Println("Done!")
  //}

  //unpack(wps_tar, wps_dir)//wps_tmp + wps_dir)
  createdir(wps_destdir)
  rename_whitespace_in_dir(wps_dir)//wps_tmp + wps_dir)

  log.Println("Copying files...Ultra slow...")
  copydir("office6", wps_destdir + "/office6")

}
