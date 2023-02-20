.PHONY: air build css
build: css
	go build -o ./build/spa ./main.go
air:
	air -build.include_ext "gcss,go,tmlp" --build.cmd "make build" --build.bin "./build/spa"
css:
	cat ./static/css/main.gcss | gcss > ./static/css/main.css
