WORKDIR=`pwd`

default: build

vet:
	go vet ./...

tools:
	go get github.com/golangci/golangci-lint/cmd/golangci-lint
	go get github.com/golang/lint/golint
	go get github.com/axw/gocov/gocov
	go get github.com/matm/gocov-html

golangci-lint:
	golangci-lint run -D errcheck 

lint:
	golint ./...

doc:
	godoc -http=:6060

deps:
	go list -f '{{ join .Deps  "\n"}}' ./... |grep "/" | grep -v "github.com/rpcxio/rpcxplus"| grep "\." | sort |uniq

fmt:
	go fmt ./...

build:
	go build ./...

test:
	go test -race ./...

cover:
	gocov test ./... | gocov-html > cover.html
	open cover.html