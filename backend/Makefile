GOCYCLO_VERSION := v0.6.0
export GOWORK := off

.PHONY: lint
lint:
	go run github.com/fzipp/gocyclo/cmd/gocyclo@$(GOCYCLO_VERSION) -over 20 -ignore '_test\.go$$' .
