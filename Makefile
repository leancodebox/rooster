.PHONY: build package-darwin test web

web:
	cd web && pnpm build

build: web
	go build ./cmd/roosterd
	go build ./cmd/rooster

package-darwin: web
	fyne package --source-dir ./cmd/rooster --target darwin --name Rooster --icon $(CURDIR)/web/public/rooster.png --app-id com.leancodebox.rooster --app-version 0.1.0 --app-build 1
	/usr/libexec/PlistBuddy -c "Add :LSUIElement bool true" Rooster.app/Contents/Info.plist

test: web
	go test ./...
	cd web && pnpm lint
