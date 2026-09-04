# StateTrak local
Backend for fetching game info from demos

Uses [DemoInfoCs](https://github.com/markus-wa/demoinfocs-golang)

## Usage

Build & run (compiles the React client into the binary, listens on `:3001`):

    make

Parse a demo offline without starting the server (writes JSON to
`controllers/data/output/local`):

    go run . -parse=path/to/demo.dem

Demos uploaded through the web UI are stored under `controllers/data/` along
with their parse output and original filename, so they survive restarts.