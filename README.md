WPS Office Installer for openSUSE

------

Installation:

    go build wps-installer.go
    go build ghost_generator.go
    go build depends_generator.go

wps-installer will be installed to /var/adm/update-scripts.

`ghost_generator -wpsdir` will generate directories and ghost files files used by specfile.

`depends_generator -wpsdir` will generate wps-office's runtime dependencies used in specfile.
