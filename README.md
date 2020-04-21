WPS Office Installer for openSUSE

------

Build:

    cd wps-office-installer
    export GO111MODULE=on
    go build

Installation: 

On openSUSE, wps-office-installer will be installed to /var/adm/update-scripts.

On other linux, copy the wps-office-installer to /usr/bin and wps.yaml to /etc/wps-office/

`wps-office-installer -install=false -ghost -dir=<dir>` will generate directories and ghost files files used by openSUSE RPM Specfile.

`wps-office-installer -install=false -dep -dir=<dir>` will generate wps-office's runtime dependencies used by openSUSE RPM Specfile.

`sudo wps-office-installer` will install the  wps-office version specified in /etc/wps-office/wps.yaml.
