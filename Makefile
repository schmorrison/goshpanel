.PHONY: build dev test lint tidy generate css css-watch clean

BINARY := bin/goshpanel
TEMPL := go run github.com/a-h/templ/cmd/templ@v0.2.778

build: generate css
	go build -o $(BINARY) ./cmd/goshpanel

dev: generate css
	go run ./cmd/goshpanel

test:
	go test ./...

lint:
	go vet ./...
	test -z "$$(gofmt -l .)"

tidy:
	go mod tidy

generate:
	$(TEMPL) generate

css:
	npm run build:css

css-watch:
	npm run watch:css

clean:
	rm -rf bin/goshpanel web/static/css/app.css
