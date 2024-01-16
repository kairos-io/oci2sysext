# oci2sysext

oci2sysext builds a [systemd-extension](https://www.freedesktop.org/software/systemd/man/latest/systemd-sysext.html) from a container image.


## Installation

### From source

```bash
make
# should produce build/oci2sysext
```

## Usage

```bash

## Create an image, and add something to it
docker build -t myimage:latest -<<EOF
FROM alpine
RUN apk add curl
EOF

## Save the image
docker save myimage:latest -o myimage.tar

## Create a systemd-extension from the image
## oci2sysext <image-tar-path> <sysext-name>
oci2sysext myimage.tar myimage

## Install the systemd-extension
mv myimage.raw /var/lib/extensions

# or symlinking
# mkdir extensions
# mv myimage.raw extensions/
# sudo ln -s $PWD/extensions /var/lib/extensions/
```