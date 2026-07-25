LDFLAGS := -ldflags="-s -w"
BINARY := mephisto

.PHONY: build run clean release

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/mephisto

run: build
	./$(BINARY) run data/faust.meph

clean:
	rm -f $(BINARY)
	rm -rf releases/

release:
	mkdir -p releases
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o releases/$(BINARY)-linux-amd64   ./cmd/mephisto
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o releases/$(BINARY)-linux-arm64   ./cmd/mephisto
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o releases/$(BINARY)-darwin-amd64  ./cmd/mephisto
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o releases/$(BINARY)-darwin-arm64  ./cmd/mephisto
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o releases/$(BINARY)-windows-amd64.exe ./cmd/mephisto
	@echo "✅ 所有平台构建完成"