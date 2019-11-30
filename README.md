WPS Office Installer for openSUSE

------

Build:

    export GO111MODULE=on
    go build wps.go

Installation: 

On openSUSE, wps-installer will be installed to /var/adm/update-scripts.

On other linux, `mv wps /usr/bin/wps-installer`.

`wps-installer -install=false -ghost -dir=<dir>` will generate directories and ghost files files used by openSUSE RPM Specfile.

`wps-installer -install=false -dep -dir=<dir>` will generate wps-office's runtime dependencies used by openSUSE RPM Specfile.

You can adjust wps-office version in wps.yaml and run wps-installer to install newer releases anytime.
