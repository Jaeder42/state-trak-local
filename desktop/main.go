package main

// StateTrak desktop app (Wails v2): the same gin API + embedded React UI as
// the standalone server, served in-process through Wails' asset server and
// shown in a native window. The client keeps using plain fetch() — every
// request that isn't a UI asset falls through to the gin router.
//
// Build: `make desktop` (cd desktop && wails build) → desktop/build/bin/.

import (
	"embed"
	"flag"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"jaeder42.tech/state-trak-local/controllers"
	"jaeder42.tech/state-trak-local/server"
	"jaeder42.tech/state-trak-local/web"
)

//go:embed all:build
var buildAssets embed.FS

func main() {
	data := flag.String("data", "", "data directory (default: the user config dir)")
	flag.Parse()

	if *data != "" {
		controllers.SetDataDir(*data)
	} else if dir, err := os.UserConfigDir(); err == nil {
		// launched from Finder the cwd is / — keep data per-user instead
		controllers.SetDataDir(filepath.Join(dir, "StateTrak"))
	}
	controllers.Init()

	gin.SetMode(gin.ReleaseMode)
	router := server.NewRouter()

	err := wails.Run(&options.App{
		Title:     "StateTrak",
		Width:     1440,
		Height:    900,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: web.Dist(), // the embedded React client (web/dist)
			// API fallback: any request that isn't a UI asset hits the gin
			// router — uploads, demo data, analyses, the BYO-key endpoints.
			Handler: router,
		},
		Bind: []interface{}{}, // no bindings — the UI talks to the HTTP API
		// Mac must be non-nil: wails v2.16 only initializes the zoomable flag
		// inside the `Mac != nil` branch, and a nil Mac leaves it at 0, which
		// disables the green maximize/fullscreen button on the window.
		Mac: &mac.Options{},
	})
	if err != nil {
		println("Error:", err.Error())
		os.Exit(1)
	}
}
