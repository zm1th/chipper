BIN := chipper
INSTALL_DIR := /usr/local/bin

.PHONY: build install clean

build:
	go build -o $(BIN) .

install: build
	mv $(BIN) $(INSTALL_DIR)/$(BIN)

clean:
	rm -f $(BIN)
