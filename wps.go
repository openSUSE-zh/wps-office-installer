package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cavaliercoder/grab"
	"github.com/marguerite/util/dir"
	"github.com/marguerite/util/fileutils"
	"github.com/marguerite/util/slice"
	yaml "gopkg.in/yaml.v2"
)

type wpsConfig struct {
	Name         string `yaml:"name"`
	Version      string `yaml:"version"`
	Alpha        string `yaml:"alpha"`
	Architecture string `yaml:"architecture"`
	URL          string `yaml:"url"`
}

func logname() string {
	ln, _ := exec.Command("/usr/bin/logname").Output()
	return string(ln)
}

func loadYAML() (wpsConfig, error) {
	file := filepath.Join("/home", logname(), ".config/wps-office/wps.yaml")
	if _, err := os.Stat(file); os.IsNotExist(err) {
		if _, err := os.Stat("wps.yaml"); os.IsNotExist(err) {
			file = "/etc/wps-office/wps.yaml"
		} else {
			file = "wps.yaml"
		}
	}

	c := wpsConfig{}

	b, err := ioutil.ReadFile(file)
	if err != nil {
		return c, err
	}

	err = yaml.Unmarshal(b, &c)
	if err != nil {
		return c, err
	}
	return c, nil
}

func download(src, dest string) error {
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		fmt.Printf("%s exists, skipped download.\n", dest)
		return nil
	}

	fmt.Printf("Downloading binary data from %s (100+ mb), it may take some time.\n", src)

	client := grab.NewClient()
	req, err := grab.NewRequest(dest, src)
	if err != nil {
		return err
	}

	resp := client.Do(req)
	fmt.Printf("\t%v\n", resp.HTTPResponse.Status)

	t := time.NewTicker(5 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			fmt.Printf("\ttransferred %d / %d bytes (%.2f%%)\n", resp.BytesComplete(), resp.Size, 100*resp.Progress())
		case <-resp.Done:
			if err = resp.Err(); err != nil {
				return err
			}
			fmt.Printf("Download Completed. Saved to %s.\n", dest)
			return nil
		}
	}
}

func unpack(src, dest, cwd string) error {
	if _, err := os.Stat(filepath.Join(dest, "usr")); !os.IsNotExist(err) {
		fmt.Printf("%s has been unpacked to %s, skipped.\n", src, dest)
		return nil
	}
	fmt.Printf("Unpacking %s to %s, it may take some time\n", src, dest)
	os.Chdir(dest)
	_, err := exec.Command("/usr/bin/unrpm", filepath.Base(src)).Output()
	if err != nil {
		return err
	}
	os.Chdir(cwd)
	fmt.Printf("%s was unpacked into %s.\n", src, dest)
	return nil
}

func renameWhitespace(parent string) {
	dirs, _ := dir.Ls(parent, "dir")
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, d := range dirs {
		if strings.Contains(d, " ") {
			os.Rename(d, filepath.Join(filepath.Dir(d), strings.Replace(filepath.Base(d), " ", "_", -1)))
		}
	}
	files, _ := dir.Ls(parent)
	for _, f := range files {
		if strings.Contains(f, " ") {
			os.Rename(f, filepath.Join(filepath.Dir(f), strings.Replace(filepath.Base(f), " ", "_", -1)))
		}
	}
}

func replaceBinaryPath(binary, path string) {
	data, _ := ioutil.ReadFile(binary)
	info, _ := os.Lstat(binary)
	re := regexp.MustCompile("(?m)^gBinPath=.*?\n")
	str := re.ReplaceAllString(string(data), "gBinPath=\""+path+"\"\n")
	ioutil.WriteFile(binary, []byte(str), info.Mode())
}

type step struct {
	name     string
	findpath string
	suffix   string
	postcmd  []string
}

func (s step) install(prefix string) {
	fmt.Printf("Processing installation - %s\n", s.name)
	files, _ := dir.Glob(filepath.Join(prefix, s.findpath), "."+s.suffix)
	for _, file := range files {
		fileutils.Copy(file, strings.Replace(file, prefix, "", 1))
	}
	if len(s.postcmd) > 0 {
		exec.Command(s.postcmd[0], s.postcmd[1:]...).Output()
	}
}

func main() {
	var directory string
	var dep, ghost, install bool
	cwd, _ := os.Getwd()
	flag.StringVar(&directory, "dir", filepath.Join(cwd, "wps-office"), "Unpacked Kingsoft WPS Office Directory.")
	flag.BoolVar(&dep, "dep", false, "Generate runtime library dependencies.")
	flag.BoolVar(&ghost, "ghost", false, "Generate Ghost files that'll be installed at runtime.")
	flag.BoolVar(&install, "install", true, "Install Kingsoft WPS Office onto your openSUSE installation.")
	flag.Parse()

	directory, _ = filepath.Abs(directory)

	if install {
		if os.Getuid() != 0 {
			fmt.Println("Root privilege is required to install wps.")
			os.Exit(1)
		}
		config, err := loadYAML()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		prefix := filepath.Join("/tmp", "wps-office_"+config.Version+"_"+config.Architecture)
		fullname := "wps-office-" + config.Version + "." + config.Architecture
		dest := filepath.Join(cwd, "usr/share/wps-office")

		if _, err = os.Stat(filepath.Join("/tmp", fullname+".txt")); !os.IsNotExist(err) {
			fmt.Printf("%s has been installed on your system, stop.\n", fullname)
			os.Exit(0)
		}

		// delete all versioned indicators
		indicators, _ := dir.Glob("/tmp", "wps-office*.txt")
		for _, indicator := range indicators {
			os.Remove(indicator)
		}

		dir.MkdirP(prefix)

		err = download(config.URL+"/"+fullname+".rpm", filepath.Join(prefix, fullname+".rpm"))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		err = unpack(filepath.Join(prefix, fullname+".rpm"), prefix, cwd)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		renameWhitespace(prefix)

		dir.MkdirP(filepath.Join(dest, "office6"))
		dir.MkdirP("/usr/share/templates/.source")
		dir.MkdirP("/etc/xdg/menus/applications-merged")

		fileutils.Copy(filepath.Join(prefix, "opt/kingsoft/wps-office/office6"), filepath.Join(dest, "office6"))

		//install stuff
		for _, bin := range []string{"wps", "wpp", "et", "wpspdf"} {
			bin = filepath.Join(prefix, "/usr/bin/"+bin)
			replaceBinaryPath(bin, dest)
			fileutils.Copy(bin, strings.Replace(bin, prefix, "", 1))
		}

		steps := []step{
			{"desktop files", "/usr/share/applications", "desktop", []string{"/usr/bin/update-desktop-database", "/usr/share/application", "&>/dev/null"}},
			{"icons", "/usr/share/icons/hicolor", "png", []string{"/usr/bin/gtk-update-icon-cache", "--quiet", "--force", "/usr/share/icons/hicolor"}},
			{"mimetypes", "/usr/share/mime/packages", "xml", []string{"/usr/bin/update-mime-database", "/usr/share/mime"}},
			{"directories", "/usr/share/desktop-directories", "directory", []string{}},
			{"templates", "/usr/share/templates", "desktop", []string{}},
		}

		for _, s := range steps {
			s.install(prefix)
		}

		fileutils.Copy(filepath.Join(prefix, "/etc/xdg/menus/applications-merged/wps-office.menu"), "/etc/xdg/menus/applications-merged/wps-office.menu")
		os.RemoveAll(prefix)
		fileutils.Touch(filepath.Join("/tmp", fullname+".txt"))
		fmt.Println("Installation Succeed.")
		os.Exit(0)
	} else {
		if _, err := os.Stat(filepath.Join(directory, "usr")); os.IsNotExist(err) {
			fmt.Println("You must specify -dir option when dep/ghost option is enabled.")
			os.Exit(1)
		}
	}

	office6dir := filepath.Join(directory, "/opt/kingsoft/wps-office/office6")
	renameWhitespace(office6dir)

	if dep {
		// dep stuff
		fmt.Printf("Finding all binaries from %s\n", office6dir)
		binaries := findBinaries(office6dir)
		raw := []string{}
		for _, binary := range binaries {
			raw = parseRawDepends(binary, raw, directory, binaries)
		}
		fmt.Println("Try resolving library dependencies to openSUSE package names.")
		depends := findDepends(raw)
		for _, d := range depends {
			fmt.Printf("\t%s\n", d)
		}
	}

	if ghost {
		// ghost stuff
		directories, _ := dir.Ls(office6dir, "dir")
		files, _ := dir.Ls(office6dir)
		ghostDirs := substitute(directories, filepath.Dir(office6dir), "%dir %{_datadir}/wps-office")
		ghostFiles := substitute(files, filepath.Dir(office6dir), "%ghost %{_datadir}/wps-office")
		dirs := substitute(directories, filepath.Dir(office6dir), "./usr/share/wps-office")

		for _, d := range ghostDirs {
			ghostFiles = append(ghostFiles, d)
		}

		for _, f := range [2]string{"%dir %{_datadir}/wps-office", "%dir %{_datadir}/wps-office/office6"} {
			ghostFiles = append(ghostFiles, f)
		}

		ioutil.WriteFile("ghostfiles.txt", []byte(strings.Join(ghostFiles, "\n")), 0644)
		ioutil.WriteFile("wps-dir.txt", []byte(strings.Join(dirs, "\n")), 0644)
	}
}

func substitute(files []string, orig string, dest string) []string {
	var res []string
	for _, f := range files {
		res = append(res, strings.Replace(f, orig, dest, -1))
	}
	return res
}

func findBinaries(directory string) []string {
	files, _ := dir.Ls(directory)
	binaries := []string{}
	for _, file := range files {
		info, _ := os.Stat(file)
		// skip zero-bit file
		if info.Size() == 0 {
			continue
		}

		// skip non-execuatable file
		if !strings.Contains(info.Mode().String(), "x") {
			continue
		}

		if !info.Mode().IsDir() && !(info.Mode()&os.ModeSymlink != 0) {
			f, _ := os.Open(file)
			defer f.Close()

			buf := make([]byte, 512)
			f.Read(buf)
			f.Seek(0, 0)

			contentType := http.DetectContentType(buf)
			if contentType == "application/octet-stream" {
				ext := filepath.Ext(file)
				if ext == "" || strings.HasSuffix(file, ".so") {
					binaries = append(binaries, file)
				}
			}
		}

	}
	return binaries
}

func parseRawDepends(file string, depends []string, wpsDir string, selfBinaries []string) []string {
	out, _ := exec.Command("ldd", file).Output()

	for _, line := range strings.Split(string(out), "\n") {
		dep := parseDepend(line, wpsDir, selfBinaries)
		if dep != "" {
			if ok, err := slice.Contains(depends, dep); !ok || err != nil {
				depends = append(depends, dep)
			}
		}
	}
	return depends
}

func parseDepend(str string, wpsDir string, binaries []string) string {
	re := regexp.MustCompile(`[[:space:]](.*)[[:space:]]=>[[:space:]]([^\(]+)`)
	if strings.Contains(str, "=>") {
		matches := re.FindStringSubmatch(str)
		if !strings.Contains(matches[2], wpsDir) {
			ok := false
			for _, bin := range binaries {
				if strings.Contains(bin, matches[1]) {
					ok = true
				}
			}
			if !ok {
				return matches[1]
			}
		}
	}
	return ""
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
		if ok, err := slice.Contains(depends, d); !ok || err != nil {
			depends = append(depends, d)
		}
	}
	return depends
}
