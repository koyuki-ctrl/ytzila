# export XDG_CACHE_HOME=/home/ainradan/goinfre/ytzila
# ARG ?= ""

build: install
	go build .

run: build
	./ytzila ${ARG}

install:
	go mod tidy

clean: 
	rm -rf go-ytdlp yt-dlp go-build 
	rm ytzila

re: clean build

format: gofmt -w .

.PHONY: build re run install