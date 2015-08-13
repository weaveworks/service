.PHONY: deps

deps:
	go get \
		github.com/golang/lint/golint \
		github.com/fzipp/gocyclo \
		github.com/mattn/goveralls \
		github.com/kisielk/errcheck
