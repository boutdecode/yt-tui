# yt-tui — build, tests, installation locale et paquet .deb
GO      ?= $(shell command -v go 2>/dev/null || echo $(HOME)/.local/go/bin/go)
VERSION ?= 0.1.0
ARCH    ?= $(shell dpkg --print-architecture 2>/dev/null || echo amd64)
PREFIX  ?= $(HOME)/.local
LDFLAGS  = -s -w -X main.version=$(VERSION)
DEB      = dist/yt-tui_$(VERSION)_$(ARCH).deb

.PHONY: build test install uninstall deb release publish clean

build:
	$(GO) build -ldflags '$(LDFLAGS)' -o yt-tui ./cmd/yt-tui

test:
	$(GO) vet ./... && $(GO) test -count=1 ./...

install: build
	install -Dm755 yt-tui $(PREFIX)/bin/yt-tui
	install -Dm644 packaging/deb/yt-tui.desktop $(PREFIX)/share/applications/yt-tui.desktop

uninstall:
	rm -f $(PREFIX)/bin/yt-tui $(PREFIX)/share/applications/yt-tui.desktop

deb:
	rm -rf dist/pkg && mkdir -p dist/pkg/DEBIAN dist/pkg/usr/bin dist/pkg/usr/share/applications
	CGO_ENABLED=0 GOARCH=$(ARCH) $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o dist/pkg/usr/bin/yt-tui ./cmd/yt-tui
	install -m644 packaging/deb/yt-tui.desktop dist/pkg/usr/share/applications/yt-tui.desktop
	sed -e 's/@VERSION@/$(VERSION)/' -e 's/@ARCH@/$(ARCH)/' packaging/deb/control.in > dist/pkg/DEBIAN/control
	find dist/pkg -type d -exec chmod 755 {} +
	chmod 755 dist/pkg/usr/bin/yt-tui
	dpkg-deb --root-owner-group --build dist/pkg $(DEB)
	rm -rf dist/pkg
	@echo "→ $(DEB)"

release:
	rm -rf dist/rel && mkdir -p dist/rel
	for a in amd64 arm64; do \
	  d=dist/rel/$$a; mkdir -p $$d; \
	  CGO_ENABLED=0 GOOS=linux GOARCH=$$a $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $$d/yt-tui ./cmd/yt-tui || exit 1; \
	  cp packaging/deb/yt-tui.desktop README.md README.fr.md $$d/; \
	  tar -czf dist/yt-tui_$(VERSION)_linux_$$a.tar.gz -C $$d yt-tui yt-tui.desktop README.md README.fr.md || exit 1; \
	done
	rm -rf dist/rel
	cd dist && sha256sum yt-tui_$(VERSION)_linux_*.tar.gz > checksums.txt
	@echo "→ dist/yt-tui_$(VERSION)_linux_{amd64,arm64}.tar.gz + dist/checksums.txt"

publish: release deb
	gh release create v$(VERSION) dist/yt-tui_$(VERSION)_linux_*.tar.gz dist/checksums.txt $(DEB) --title "v$(VERSION)" --generate-notes

clean:
	rm -rf dist yt-tui
