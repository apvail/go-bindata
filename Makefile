.PHONY: all clean install

all:
	make -C testdata

./bin/go-bindata:
	make -C testdata ../bin/go-bindata

install:
	go install ./...

clean: ./bin/go-bindata
	rm ./bin/go-bindata
