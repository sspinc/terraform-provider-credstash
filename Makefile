PACKAGES = $(shell go list ./... | grep -v vendor)

install:
	go install -v

build:
	go build -v -i -o terraform-provider-credstash

test:
	go test $(TESTOPTS) $(PACKAGES)

release:
	rm -f -R bin
	GOOS=darwin go build -v -o bin/darwin/terraform-provider-credstash_v$(version)_x4
	GOOS=linux go build -v -o bin/linux/terraform-provider-credstash_v$(version)_x4

.DEFAULT: build
