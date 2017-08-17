.PHONY: all linux darwin

all: linux darwin

clean:
	rm -f machine-Linux-* machine-Darwin-*

linux:
	env CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o machine-Linux-x86_64 .

darwin:
	env CGO_ENABLED=0 GOOS=darwin go build -a -installsuffix cgo -o machine-Darwin-x86_64 .
