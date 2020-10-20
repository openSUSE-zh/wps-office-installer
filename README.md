WPS Office Installer for openSUSE

------

Build:

    cd wps-office-installer
    export GO111MODULE=on
    go mod download
    go mod vendor
    go build -o wps-office-installer wps.go

Installation:

On openSUSE, `sudo zypper in wps-office`

On other linux, copy the wps-office-installer to /usr/bin and wps.yaml to /etc/wps-office/

`wps-office-installer -g -d=<dir>` will generate directories and ghost files files used by openSUSE RPM Specfile.

`wps-office-installer -r -d=<dir>` will generate wps-office's runtime dependencies used by openSUSE RPM Specfile.

`sudo wps-office-installer -i` will install the  wps-office version specified in /etc/wps-office/wps.yaml.
