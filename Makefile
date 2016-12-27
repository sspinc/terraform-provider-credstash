PACKAGES = $(shell go list ./... | grep -v vendor)

build:
	go build -v -o terraform-provider-credstash

test:
	go test $(TESTOPTS) $(PACKAGES)

.DEFAULT: build
