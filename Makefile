CC = go
TARGET = trivial
BIN = ./bin

.PHONY: build clean

$(TARGET):
	$(CC) build -o $(BIN)

build: $(TARGET)

clean:
	rm -rf $(BIN)/$(TARGET)

