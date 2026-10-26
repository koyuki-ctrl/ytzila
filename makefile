build: install
	go build .

install:
	go mod tidy

clean: 
	rm -rf go-ytdlp yt-dlp go-build 
	rm ytzila

re: clean build

format:
	gofmt -w .

.PHONY: build re install