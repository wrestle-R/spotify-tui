.PHONY: build test check package package-check clean

VERSION ?= 0.5.1

build:
	mkdir -p dist
	cd go && CGO_ENABLED=0 go build -trimpath -ldflags '-s -w -X main.version=$(VERSION)' -o ../dist/spotify-tui ./cmd/spotify-tui
	cargo build --manifest-path rust/Cargo.toml --locked --release
	install -m 755 rust/target/release/spotify-tui-playback dist/.spotify-tui-playback.new
	mv -f dist/.spotify-tui-playback.new dist/spotify-tui-playback

test:
	cd go && go test -race ./...
	cargo test --manifest-path rust/Cargo.toml --locked

check:
	test -z "$$(gofmt -l go/cmd go/internal)"
	cd go && go vet ./...
	cargo fmt --manifest-path rust/Cargo.toml --check
	cargo clippy --manifest-path rust/Cargo.toml --locked --all-targets -- -D warnings

package: build
	python3 scripts/release.py package --version $(VERSION)

package-check: package
	python3 scripts/tests/test-release.py

clean:
	rm -rf dist
