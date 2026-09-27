.PHONY: build run client clean

build: client
	go build -o statetrak .

run: build
	@lsof -ti:3007 | xargs kill -9 2>/dev/null || true
	./statetrak

client:
	cd client && npm run build
	rm -rf web/dist
	cp -r client/build web/dist
	touch web/dist/.gitkeep

clean:
	rm -f statetrak
	rm -rf web/dist
