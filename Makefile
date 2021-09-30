SERVER_NAME := mgw

default: build
	./dist/debug

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -a -ldflags "-s -w" -trimpath -gcflags="all=-trimpath=${PWD}" -asmflags="all=-trimpath=${PWD}"  -o dist/linux/$(SERVER_NAME) cmd/main.go

macos:
	go build -v -a -ldflags "-s -w" -trimpath -gcflags="all=-trimpath=${PWD}" -asmflags="all=-trimpath=${PWD}"  -o dist/macOS/$(SERVER_NAME) cmd/main.go

build:
	go build -o dist/debug cmd/main.go