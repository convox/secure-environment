all: build

build:
	mkdir -p builds/linux
	GOOS=linux go build -o builds/linux/secure-environment
