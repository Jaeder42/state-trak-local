package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/joho/godotenv"
	"jaeder42.tech/state-trak-local/controllers"
	"jaeder42.tech/state-trak-local/server"
)

// openBrowser launches the default browser at url — the standalone-app UX:
// run the binary, the UI appears. Opt out with STATETRAK_NO_OPEN=1 (used by
// the systemd unit on the server, where there is no browser to open).
// For the packaged desktop window see desktop/ (Wails).
func openBrowser(url string) {
	if os.Getenv("STATETRAK_NO_OPEN") != "" {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start() // best effort — headless boxes just fail quietly
}

func main() {
	// Optional .env in the repo root (gitignored) — e.g. TYPESAFE_API_KEY.
	// Real environment variables take precedence.
	_ = godotenv.Load()

	parse := flag.String("parse", "", "parse the given .dem file and exit (writes JSON to controllers/data/output/local)")
	jev := flag.String("jev", "", "analyze a parsed demo's round economies with JEV (eco/force/full detection); argument is a demo id, e.g. -jev=local")
	addr := flag.String("addr", ":3007", "listen address")
	data := flag.String("data", "", "data directory for uploads + parse output (default: ./controllers/data)")
	flag.Parse()

	if *data != "" {
		controllers.SetDataDir(*data)
	}

	if *parse != "" {
		controllers.ParseDemo("local", *parse)
		return
	}

	if *jev != "" {
		if err := controllers.AnalyzeEconomy(*jev); err != nil {
			fmt.Println("analysis failed:", err)
			os.Exit(1)
		}
		return
	}

	controllers.Init()
	router := server.NewRouter()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	url := fmt.Sprintf("http://localhost%s", portOf(ln))
	fmt.Println("StateTrak listening on", url)
	go func() {
		// give the server a moment to be ready, then pop the UI
		time.Sleep(300 * time.Millisecond)
		openBrowser(url)
	}()
	if err := router.RunListener(ln); err != nil {
		fmt.Println(err)
	}
}

// portOf formats the bound address as ":<port>" for display.
func portOf(ln net.Listener) string {
	addr := ln.Addr().String()
	if _, port, err := net.SplitHostPort(addr); err == nil {
		return ":" + port
	}
	return addr
}
