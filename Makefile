
GOCMD:=go
VERSION:=$(shell git describe --always)
PACKAGES:=$(shell go list ./... | grep -v /vendor/)
GO15VENDOREXPERIMENT=1

.PHONY: all test clean rpm

all: disadis 

disadis: $(wildcard *.go)
	go build .

test:
	$(GOCMD)  test -v $(PACKAGES)

clean:
	        rm -f disadis 

rpm: disadis
	               fpm -t rpm -s dir \
	               --name disadis \
	                --version $(VERSION) \
	                --vendor ndlib \
	                --maintainer DLT \
	                --description "disadis daemon" \
	                --rpm-user app \
	                --rpm-group app \
			noids=/opt/disadis/bin/disadis
