all: build oci2sysext

build:
	mkdir -p build

oci2sysext:
	go build -o build/oci2sysext ./main.go 

clean:
	rm -rf build