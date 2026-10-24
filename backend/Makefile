GOCYCLO_VERSION := v0.6.0
APP := ./cmd/prisoners-of-war
.DEFAULT_GOAL := build
export GOWORK := off

.PHONY: run build test lint clean
run:
	go run $(APP) $(ARGS)

build:
	go build -o bin/prisoners-of-war $(APP)

test:
	go test -race ./...

lint:
	go run github.com/fzipp/gocyclo/cmd/gocyclo@$(GOCYCLO_VERSION) -over 20 -ignore '_test\.go$$' .

clean:
	rm -rf bin
