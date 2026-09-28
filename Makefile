.PHONY: build run client clean release desktop dmg

# Plain server binary (browser mode) — also what the release targets build.
build: client
	go build -o statetrak .

run: build
	@lsof -ti:3007 | xargs kill -9 2>/dev/null || true
	STATETRAK_NO_OPEN=1 ./statetrak

client:
	cd client && npm run build
	rm -rf web/dist
	cp -r client/build web/dist
	touch web/dist/.gitkeep

# Wails desktop app -> desktop/build/bin/StateTrak.app (needs the wails CLI:
# go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0). Native window,
# closing it quits the app.
WAILS ?= $(shell command -v wails 2>/dev/null || echo $(HOME)/go/bin/wails)
desktop: client
	cd desktop && $(WAILS) build

# .dmg of the desktop app into release/
dmg: desktop
	mkdir -p release
	rm -f release/StateTrak.dmg
	hdiutil create -volname StateTrak -srcfolder desktop/build/bin/StateTrak.app -ov -format UDZO release/StateTrak.dmg >/dev/null
	@ls -la release/StateTrak.dmg

clean:
	rm -f statetrak
	rm -rf web/dist release desktop/build/bin

# Cross-compiled standalone binaries into release/ — single files with the
# UI embedded; they open the default browser (pure Go, no cgo — build the
# Wails app natively per OS for the windowed desktop app instead).
release: client
	rm -rf release
	mkdir -p release
	GOOS=darwin  GOARCH=arm64 go build -o release/statetrak-macos-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -o release/statetrak-macos-intel .
	GOOS=linux   GOARCH=amd64 go build -o release/statetrak-linux-amd64 .
	GOOS=windows GOARCH=amd64 go build -o release/statetrak-windows-amd64.exe .
	@ls -la release/