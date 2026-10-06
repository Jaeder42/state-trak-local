package controllers

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type DemoStatus struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"` // "parsing" | "done" | "error"
	Error    string `json:"error,omitempty"`
	Progress int    `json:"progress"`
	Total    int    `json:"total"`
}

var (
	demoMu     sync.Mutex
	demoStatus = map[string]*DemoStatus{}

	// parseSem caps concurrent demo parses — demoinfocs on a large demo
	// allocates heavily, and a burst of uploads would otherwise spike RAM.
	parseSem chan struct{}
)

const (
	defaultUploadDir = "./controllers/data/uploads"
	defaultOutputDir = "./controllers/data/output"
	defaultDataDir   = "./controllers/data"

	defaultUploadMaxBytes = 1 << 30 // 1 GB
)

// Actual data locations. Defaults keep the repo layout; standalone binaries
// can relocate them via SetDataDir (-data flag) before Init() runs.
var (
	uploadDir = defaultUploadDir
	outputDir = defaultOutputDir
	dbFile    = filepath.Join(defaultDataDir, "statetrak.db")
)

// SetDataDir moves uploads + parse output + the database under one root
// directory.
func SetDataDir(dir string) {
	uploadDir = filepath.Join(dir, "uploads")
	outputDir = filepath.Join(dir, "output")
	dbFile = filepath.Join(dir, "statetrak.db")
}

// Init prepares the data dirs, opens the sqlite store and restores the
// persisted demo list. Called from main (after flag parsing) instead of an
// init() so that -data can take effect first.
func Init() {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		panic(err)
	}

	// Parse-time storage: every entry point (upload, -parse, -reparse)
	// writes through the store, so it opens before anything can parse.
	s, err := OpenStore(dbFile)
	if err != nil {
		panic(err)
	}
	db = s
	// A "parsing" row left by a crash can never finish — flip it before
	// anything is served. The .dem upload is kept, so a re-parse recovers.
	if err := db.SweepInterrupted(); err != nil {
		panic(err)
	}
	// One-shot migration: bring demos parsed by pre-sqlite builds (the old
	// per-demo output/<id>/ trees) into the store. Already-imported demos
	// are skipped, so this is cheap after the first boot.
	if n, err := db.ImportLegacyOutput(outputDir); err != nil {
		panic(err)
	} else if n > 0 {
		fmt.Println("imported", n, "legacy demo(s) into the sqlite store")
	}

	concurrency := 1
	if v := os.Getenv("PARSE_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			concurrency = n
		}
	}
	parseSem = make(chan struct{}, concurrency)
	loadPersistedDemos()
}

// demoDir returns the output directory for a demo id.
// filepath.Base guards against path traversal via the id route param.
func demoDir(id string) string {
	return filepath.Join(outputDir, filepath.Base(id))
}

// UploadPath returns where a demo's original .dem is stored. The
// `-reparse=<id>` flag re-runs the parser on it to refresh stale output
// (e.g. demos parsed before a newer parser learned rosters/economy).
func UploadPath(id string) string {
	return filepath.Join(uploadDir, filepath.Base(id)+".dem")
}

// demoMeta is the old meta.json shape — still read by the legacy importer
// (store.go) to recover original filenames from pre-sqlite demo trees.
type demoMeta struct {
	Name string `json:"name"`
}

// loadPersistedDemos restores the status of previously parsed demos on
// startup — the store is the persistence now (uploaded names, final status
// and parse errors all live in the demos table).
func loadPersistedDemos() {
	rows, err := db.ListDemos()
	if err != nil {
		panic(err)
	}
	for _, d := range rows {
		s := &DemoStatus{Id: d.Id, Name: d.Name, Status: d.Status, Error: d.Error}
		if d.Status == "done" {
			s.Progress, s.Total = 100, 100
		}
		demoStatus[d.Id] = s
	}
}

// updateProgress reports parse progress for a demo, if it is being tracked.
func updateProgress(demoId string, progress int, total int) {
	demoMu.Lock()
	defer demoMu.Unlock()
	if s, ok := demoStatus[demoId]; ok {
		s.Progress = progress
		s.Total = total
	}
}

func UploadDemo(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "missing file"})
		return
	}
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".dem") {
		c.JSON(400, gin.H{"error": "file must be a .dem"})
		return
	}
	maxBytes := int64(defaultUploadMaxBytes)
	if v := os.Getenv("UPLOAD_MAX_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			maxBytes = n
		}
	}
	if file.Size > maxBytes {
		c.JSON(400, gin.H{"error": fmt.Sprintf("file too large (max %d MB)", maxSizeMB(maxBytes))})
		return
	}

	demoId := fmt.Sprintf("%d", time.Now().UnixNano())
	demoPath := filepath.Join(uploadDir, demoId+".dem")
	if err := c.SaveUploadedFile(file, demoPath); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Storage row for the parse (status "parsing" until ParseDemo finishes).
	// The original filename lives in the row — the store is the persistence.
	if err := db.CreateDemo(demoId, file.Filename); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	demoMu.Lock()
	demoStatus[demoId] = &DemoStatus{
		Id:     demoId,
		Name:   file.Filename,
		Status: "parsing",
	}
	demoMu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				demoMu.Lock()
				demoStatus[demoId].Status = "error"
				demoStatus[demoId].Error = fmt.Sprintf("%v", r)
				demoMu.Unlock()
				// Same fate in the database (best effort — the parse is over).
				_ = db.FailDemo(demoId, fmt.Sprintf("%v", r))
			}
		}()
		parseSem <- struct{}{} // wait for a free parse slot (RAM cap)
		defer func() { <-parseSem }()
		ParseDemo(demoId, demoPath)
		demoMu.Lock()
		demoStatus[demoId].Status = "done"
		demoStatus[demoId].Progress = 100
		demoStatus[demoId].Total = 100
		demoMu.Unlock()
	}()

	c.JSON(200, gin.H{"id": demoId})
}

// maxSizeMB renders an upload cap for error messages.
func maxSizeMB(bytes int64) int64 {
	return bytes / (1 << 20)
}

func GetDemoStatus(c *gin.Context) {
	id := c.Param("id")
	demoMu.Lock()
	s, ok := demoStatus[id]
	demoMu.Unlock()
	if !ok {
		c.JSON(404, gin.H{"error": "demo not found"})
		return
	}
	c.JSON(200, s)
}

func GetOutput(c *gin.Context) {
	data, ok, err := db.GetDemoOutput(c.Param("id"))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(404, gin.H{"error": "demo not found"})
		return
	}
	c.Data(200, "application/json", data)
}

func DeleteDemo(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "missing id"})
		return
	}

	demoMu.Lock()
	delete(demoStatus, id)
	demoMu.Unlock()

	// Rounds, kills and caches cascade away with the row.
	_, _ = db.DeleteDemo(id)

	os.RemoveAll(demoDir(id))
	os.Remove(filepath.Join(uploadDir, filepath.Base(id)+".dem"))

	c.JSON(200, gin.H{"ok": true})
}

func ListDemos(c *gin.Context) {
	type DemoInfo struct {
		Id     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	demos := []DemoInfo{}
	demoMu.Lock()
	for _, s := range demoStatus {
		demos = append(demos, DemoInfo{Id: s.Id, Name: s.Name, Status: s.Status})
	}
	demoMu.Unlock()
	sort.Slice(demos, func(i, j int) bool { return demos[i].Id > demos[j].Id })
	c.JSON(200, demos)
}
