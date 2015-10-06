.PHONY: deps clean

deps:
	go get \
		github.com/golang/lint/golint \
		github.com/fzipp/gocyclo \
		github.com/mattn/goveralls \
		github.com/kisielk/errcheck

clean:
	for dir in app-mapper client users frontend monitoring; do make -C $$dir clean; done
