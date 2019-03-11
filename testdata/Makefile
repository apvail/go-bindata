FILES:=$(wildcard out/*.go)

.PHONY: check errcheck golint shadow tools

check: $(FILES:.go=.checked)

out/%.checked: out/%.go
	errcheck $<
	go vet --all $<
	go vet -vettool=$(which shadow) $<
	golint $<

../bin/go-bindata: $(wildcard ../*.go) $(wildcard ../**/*.go)
	go build -o ../bin/go-bindata ../go-bindata/*.go

out/compress-memcopy.go: $(wildcard in/**/*) ../bin/go-bindata
	../bin/go-bindata -o $@ in/...

out/compress-nomemcopy.go: $(wildcard in/**/*) ../bin/go-bindata
	../bin/go-bindata -nomemcopy -o $@ in/...

out/debug.go: $(wildcard in/**/*) ../bin/go-bindata
	../bin/go-bindata -debug -o $@ in/...

out/nocompress-memcopy.go: $(wildcard in/**/*) ../bin/go-bindata
	../bin/go-bindata -nocompress -o $@ in/...

out/nocompress-nomemcopy.go: $(wildcard in/**/*) ../bin/go-bindata
	../bin/go-bindata -nocompress -nomemcopy -o $@ in/...

#Tools needed for testing
tools: errcheck shadow golint 

errcheck:
	go get -u github.com/kisielk/errcheck

shadow:
	go install golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow

golint:
	go get -u github.com/golang/lint
