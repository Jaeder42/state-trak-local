package controllers

import (
	"encoding/json"
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

	defaultUploadMaxBytes = 1 << 30 // 1 GB
)

// Actual data locations. Defaults keep the repo layout; standalone binaries
// can relocate them via SetDataDir (-data flag) before Init() runs.
var (
	uploadDir = defaultUploadDir
	outputDir = defaultOutputDir
)

// SetDataDir moves uploads + parse output under one root directory.
func SetDataDir(dir string) {
	uploadDir = filepath.Join(dir, "uploads")
	outputDir = filepath.Join(dir, "output")
}

// Init prepares the data dirs and the persisted demo list. Called from
// main (after flag parsing) instead of an init() so that -data can take
// effect first.
func Init() {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		panic(err)
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

// demoMeta is persisted next to the parse output so demo names survive restarts.
type demoMeta struct {
	Name string `json:"name"`
}

// loadPersistedDemos restores the status of previously parsed demos on startup.
func loadPersistedDemos() {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		s := &DemoStatus{Id: id, Name: id, Status: "done", Progress: 100, Total: 100}
		if data, err := os.ReadFile(filepath.Join(demoDir(id), "meta.json")); err == nil {
			var meta demoMeta
			if json.Unmarshal(data, &meta) == nil && meta.Name != "" {
				s.Name = meta.Name
			}
		}
		if _, err := os.Stat(filepath.Join(demoDir(id), "output.json")); err != nil {
			s.Status = "error"
			s.Error = "parsing incomplete (output.json missing)"
			s.Progress = 0
			s.Total = 0
		}
		demoStatus[id] = s
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

	// Persist the original filename so the demo list survives restarts.
	if err := os.MkdirAll(demoDir(demoId), 0755); err == nil {
		if meta, err := json.Marshal(demoMeta{Name: file.Filename}); err == nil {
			os.WriteFile(filepath.Join(demoDir(demoId), "meta.json"), meta, 0644)
		}
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
	filename := filepath.Join(demoDir(c.Param("id")), "output.json")
	if _, err := os.Stat(filename); err != nil {
		c.JSON(404, gin.H{"error": "demo not found"})
		return
	}
	c.File(filename)
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

	os.RemoveAll(demoDir(id))
	os.Remove(filepath.Join(uploadDir, filepath.Base(id)+".dem"))

	c.JSON(200, gin.H{"ok": true})
}

func ListDemos(c *gin.Context) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	type DemoInfo struct {
		Id     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	demos := []DemoInfo{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		demoMu.Lock()
		s := demoStatus[id]
		demoMu.Unlock()
		name := id
		status := "done"
		if s != nil {
			name = s.Name
			status = s.Status
		}
		demos = append(demos, DemoInfo{Id: id, Name: name, Status: status})
	}
	sort.Slice(demos, func(i, j int) bool { return demos[i].Id > demos[j].Id })
	c.JSON(200, demos)
}
