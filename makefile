# export XDG_CACHE_HOME=/home/ainradan/goinfre/ytzila
# ARG ?= ""

# run:
#	./ytzila ${ARG}

build: install
	go build .

install:
	go mod tidy

clean:
	rm -rf go-ytdlp yt-dlp go-build
	rm ytzila

re: clean build

.PHONY: build re run install