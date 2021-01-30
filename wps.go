package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/marguerite/go-stdlib/dir"
	"github.com/marguerite/go-stdlib/fileutils"
	"github.com/marguerite/go-stdlib/httputils"
	"github.com/marguerite/go-stdlib/runtime"
	"github.com/urfave/cli"
	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	Name         string `yaml:"name"`
	Version      string `yaml:"version"`
	Alpha        string `yaml:"alpha"`
	Architecture string `yaml:"architecture"`
	URL          string `yaml:"url"`
}

func load() (config Config, err error) {
	file := filepath.Join("/home", runtime.LogName(), ".config/wps-office/wps.yaml")
	if _, err = os.Stat(file); os.IsNotExist(err) {
		if _, err = os.Stat("wps.yaml"); os.IsNotExist(err) {
			file = "/etc/wps-office/wps.yaml"
		} else {
			file = "wps.yaml"
		}
	}

	b, err := ioutil.ReadFile(file)
	if err != nil {
		return config, err
	}

	err = yaml.Unmarshal(b, &config)
	if err != nil {
		return config, err
	}
	return config, nil
}

func download(src, dest string) error {
	fmt.Printf("Downloading binary data from %s (200+ mb), it may take some time.\n", src)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command("/usr/bin/aria2c", "-c", "--check-certificate=false", "-x", "16", "-o", dest, src)
	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()

	var errStdout, errStderr error
	stdout := io.MultiWriter(os.Stdout, &stdoutBuf)
	stderr := io.MultiWriter(os.Stderr, &stderrBuf)
	err := cmd.Start()
	if err != nil {
		fmt.Printf("cmd.Start() failed with %s\n", err)
		return err
	}
	go func() {
		_, errStdout = io.Copy(stdout, stdoutIn)
	}()
	go func() {
		_, errStderr = io.Copy(stderr, stderrIn)
	}()
	err = cmd.Wait()
	if err != nil {
		fmt.Printf("cmd.Run() failed with %s\n", err)
		return err
	}
	if errStdout != nil {
		fmt.Println("failed to capture stdout")
		return errStdout
	}
	if errStderr != nil {
		fmt.Println("failed to capture stderr")
		return errStderr
	}
	outStr, errStr := string(stdoutBuf.Bytes()), string(stderrBuf.Bytes())
	fmt.Printf("\nout:\n%s\nerr:\n%s\n", outStr, errStr)
	return nil
}

func unpack(src, dest string) error {
	if _, err := os.Stat(filepath.Join(dest, "usr")); !os.IsNotExist(err) {
		fmt.Printf("%s has been unpacked to %s, skipped.\n", src, dest)
		return nil
	}
	fmt.Printf("Unpacking %s to %s, it may take some time\n", src, dest)
	cwd, _ := os.Getwd()
	os.Chdir(dest)
	_, err := exec.Command("/usr/bin/unrpm", filepath.Base(src)).Output()
	if err != nil {
		return err
	}
	os.Chdir(cwd)
	fmt.Printf("%s was unpacked into %s.\n", src, dest)
	return nil
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
	files, _ := dir.Glob("."+s.suffix, filepath.Join(prefix, s.findpath))
	for _, file := range files {
		fileutils.Copy(file, strings.Replace(file, prefix, "", 1))
	}
	if len(s.postcmd) > 0 {
		exec.Command(s.postcmd[0], s.postcmd[1:]...).Output()
	}
}

func main() {
	cli.VersionFlag = cli.BoolFlag{
		Name:  "version",
		Usage: "Display version and exit.",
	}
	app := cli.NewApp()
	app.Usage = "WPS Office installer for openSUSE"
	app.Description = "Install WPS Office in your openSUSE easily."
	app.Version = "20210130"
	app.Authors = []cli.Author{
		{Name: "Marguerite Su", Email: "marguerite@opensuse.org"},
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "dir, d",
			Value: "./opt/kingsoft/wps-office",
			Usage: "Unpacked Kingsoft WPS Office Directory.",
		},
		cli.BoolFlag{
			Name:  "gen-runtime-deps, r",
			Usage: "Generate runtime library dependencies.",
		},
		cli.BoolFlag{
			Name:  "gen-ghost, g",
			Usage: "Generate Ghost files for RPM Specfile.",
		},
		cli.BoolFlag{
			Name:  "install, i",
			Usage: "Install WPS Office to your openSUSE.",
		},
	}

	app.Action = func(c *cli.Context) error {

		if c.Bool("i") {
			if os.Getuid() != 0 {
				fmt.Println("root privilege is required to install wps.")
				os.Exit(1)
			}

			config, err := load()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			prefix := filepath.Join("/tmp", "wps-office_"+config.Version+"_"+config.Architecture)
			fullname := "wps-office-" + config.Version + "." + config.Architecture
			rpm := filepath.Join(prefix, fullname+".rpm")
			dest := "/usr/share/wps-office"

			if _, err = os.Stat(filepath.Join("/tmp", fullname+".txt")); !os.IsNotExist(err) {
				fmt.Printf("%s has been installed on your system, stop.\n", fullname)
				os.Exit(0)
			}

			// delete all versioned indicators
			indicators, _ := dir.Glob("/tmp/wps-office-*.txt", "/tmp")
			for _, indicator := range indicators {
				os.Remove(indicator)
			}

			for _, d := range []string{filepath.Join(dest, "office6"), "/usr/share/templates/.source", "/etc/xdg/menus/applications-merged"} {
				if _, err := os.Stat(d); !os.IsNotExist(err) {
					os.RemoveAll(d)
				}
				dir.MkdirP(d)
			}

			dir.MkdirP(prefix)

			fi, err := os.Stat(rpm)

			resp, err1 := http.Head(config.URL + "/" + fullname + ".rpm")
			if err1 != nil {
				fmt.Println("failed to establish connection to wps-office download server.")
				os.Exit(1)
			}

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("expected connection status ok, got %d\n", resp.StatusCode)
				os.Exit(1)
			}

			size, _ := strconv.Atoi(resp.Header.Get("Content-Length"))

			if os.IsNotExist(err) || fi.Size() != int64(size) {
				os.RemoveAll(rpm)
				err = download(config.URL+"/"+fullname+".rpm", rpm)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
			} else {
				fmt.Printf("%s has already downloaded, skip downloading...\n", rpm)
			}

			err = unpack(rpm, prefix)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			fileutils.Copy(filepath.Join(prefix, "opt/kingsoft/wps-office/office6"), filepath.Join(dest, "office6"))

			//install stuff
			for _, bin := range []string{"wps", "wpp", "et", "wpspdf"} {
				bin = filepath.Join(prefix, "/usr/bin", bin)
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
			if _, err := os.Stat(c.String("d")); os.IsNotExist(err) {
				fmt.Println("You must specify -d option when gen-runtime-deps/gen-ghost option is enabled.")
				os.Exit(1)
			}
		}

		office6dir := filepath.Join(c.String("d"), "office6")

		if c.Bool("r") {
			// dep stuff
			fmt.Printf("Finding all binaries from %s\n", office6dir)
			binaries := findBinaries(office6dir)
			libraries := make(map[string]struct{})
			for _, binary := range binaries {
				parseLibraries(binary, libraries, c.String("d"), binaries)
			}
			fmt.Println("Try resolving library dependencies to openSUSE package names.")
			dependencies := findDependencies(libraries)
			for k := range dependencies {
				fmt.Printf("%s\n", k)
			}
		}

		if c.Bool("g") {
			// ghost stuff
			matches, _ := dir.Ls(office6dir, true, true)
			var files, directories []string

			for _, m := range matches {
				i, _ := os.Stat(m)
				if i.IsDir() {
					directories = append(directories, m)
				} else {
					files = append(files, m)
				}
			}

			re := regexp.MustCompile(`[^\/]+\s+[^\/]+`)

			dirs := substitute(directories, filepath.Dir(office6dir), "./usr/share/wps-office")

			for _, x := range [][]string{files, directories} {
				for i, y := range x {
					if re.MatchString(y) {
						x[i] = "\"" + y + "\""
					}
				}
			}

			ghostDirs := substitute(directories, filepath.Dir(office6dir), "%dir %{_datadir}/wps-office")
			ghostFiles := substitute(files, filepath.Dir(office6dir), "%ghost %{_datadir}/wps-office")

			for _, d := range ghostDirs {
				ghostFiles = append(ghostFiles, d)
			}

			for _, f := range [2]string{"%dir %{_datadir}/wps-office", "%dir %{_datadir}/wps-office/office6"} {
				ghostFiles = append(ghostFiles, f)
			}

			ioutil.WriteFile("ghostfiles.txt", []byte(strings.Join(ghostFiles, "\n")), 0644)
			ioutil.WriteFile("wps-dir.txt", []byte(strings.Join(dirs, "\n")), 0644)
		}

		return nil
	}

	_ = app.Run(os.Args)
}

func substitute(files []string, orig string, dest string) (result []string) {
	var dest1 string
	for _, f := range files {
		if strings.HasPrefix(f, "\"") {
			if strings.HasPrefix(dest, "%dir") {
				f = strings.TrimPrefix(f, "\"")
				dest1 = strings.Replace(dest, "%dir %", "%dir \"%", 1)
			}
			if strings.HasPrefix(dest, "%ghost") {
				f = strings.TrimPrefix(f, "\"")
				dest1 = strings.Replace(dest, "%ghost %", "%ghost \"%", 1)
			}
			result = append(result, strings.Replace(f, orig, dest1, -1))
		} else {
			result = append(result, strings.Replace(f, orig, dest, -1))
		}
	}
	return result
}

func findBinaries(directory string) (binaries []string) {
	files, _ := dir.Ls(directory, true, true)
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

func parseLibraries(file string, libraries map[string]struct{}, wpsDir string, selfBinaries []string) {
	out, _ := exec.Command("ldd", file).Output()

	for _, line := range strings.Split(string(out), "\n") {
		library := parseLibrary(line, wpsDir, selfBinaries)
		if len(library) > 0 {
			if _, ok := libraries[library]; !ok {
				if library == "libQtXml.so.4" {
					fmt.Printf("libQtXml.so.4 required by %s may not find on openSUSE Tumbleweed since Qt4 was deprecated\n", strings.TrimPrefix(file, wpsDir))
				}
				libraries[library] = struct{}{}
			}
		}
	}
}

func parseLibrary(line string, wpsDir string, binaries []string) (library string) {
	re := regexp.MustCompile(`[[:space:]](.*)[[:space:]]=>[[:space:]]([^\(]+)`)
	if strings.Contains(line, "=>") {
		matches := re.FindStringSubmatch(line)
		if !strings.Contains(matches[2], wpsDir) {
			ok := false
			for _, v := range binaries {
				if strings.Contains(v, matches[1]) {
					ok = true
				}
			}
			if !ok {
				return matches[1]
			}
		}
	}
	return library
}

// findDependencies find wps runtime dependencies through zypper or rpmfind.net
func findDependencies(libraries map[string]struct{}) map[string]struct{} {
	packages := make(map[string]struct{})
	support64 := runtime.Is64Bit()
	client := httputils.ProxyClient()
	version := openSUSEversion()

	wg := sync.WaitGroup{}
	wg.Add(len(libraries))
	mux := sync.Mutex{}
	ch := make(chan struct{}, 5)

	for library := range libraries {
		go func(library, version string, packages map[string]struct{}, client *http.Client, support64 bool) {
			defer wg.Done()
			defer func() { <-ch }() // release chan
			ch <- struct{}{}        // acquire chan
			out, err := exec.Command("/usr/bin/zypper", "--no-refresh", "se", "-f", "-i", library).Output()

			// query from rpmfind.net
			if err != nil {
				pkg := queryPackage(client, library, version, support64)
				if len(pkg) == 0 {
					fmt.Printf("Can't resolve %s from rpmfind.net for openSUSE version %s, 64bit %v, may be not supported at all?\n", library, version, support64)
				} else {
					if _, ok := packages[pkg]; !ok {
						mux.Lock()
						packages[pkg] = struct{}{}
						mux.Unlock()
					}
				}
			}

			// can find from installed packages
			scanner := bufio.NewScanner(bytes.NewReader(out))
			for scanner.Scan() {
				if strings.HasPrefix(scanner.Text(), "i") {
					pkg := strings.TrimSpace(strings.Split(scanner.Text(), "|")[1])
					// don't self requires
					if pkg == "wps-office" {
						break
					}
					if !support64 || !strings.Contains(pkg, "-32bit") {
						if _, ok := packages[pkg]; !ok {
							mux.Lock()
							packages[pkg] = struct{}{}
							mux.Unlock()
							break
						}
					}
				}
			}
		}(library, version, packages, client, support64)
	}

	wg.Wait()

	return packages
}

// openSUSEversion() return openSUSE version like "Tumbleweed" or "Leap 15.2"
func openSUSEversion() (version string) {
	return strings.TrimLeft(runtime.LinuxDistribution(), "openSUSE ")
}

// queryPackage query rpm package name from rpmfind.net
func queryPackage(c *http.Client, library, version string, support64 bool) (pkg string) {
	url := "https://rpmfind.net/linux/rpm2html/search.php?query=" + library
	if support64 {
		url += "%28%29%2864bit%29"
	}
	url += "&submit=Search+...&system=opensuse&arch="
	if support64 {
		url += "x86_64"
	} else {
		url += "i586"
	}

	resp, err := c.Get(url)
	if err != nil {
		resp.Body.Close()
		panic(err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		panic(err)
	}

	var tmp []string

	doc.Find("td").Each(func(i int, s *goquery.Selection) {
		if i > 11 && i%2 == 0 {
			tmp = append(tmp, s.Text())
		}
	})

	re := regexp.MustCompile(`^(.*?)-\d+\.`)

	for i, v := range tmp {
		if i > 0 && i%2 != 0 && strings.Contains(v, version) {
			pkg = tmp[i-1]
			pkg = re.FindStringSubmatch(pkg)[1]
			return pkg
		}
	}

	return pkg
}
