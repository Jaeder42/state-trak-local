package controllers

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
)

const (
	uploadDir = "./controllers/data/uploads"
	outputDir = "./controllers/data/output"
)

func init() {
	os.MkdirAll(uploadDir, 0755)
	os.MkdirAll(outputDir, 0755)
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

	demoId := fmt.Sprintf("%d", time.Now().UnixNano())
	demoPath := filepath.Join(uploadDir, demoId+".dem")
	if err := c.SaveUploadedFile(file, demoPath); err != nil {
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
			}
		}()
		ParseDemo(demoId, demoPath)
		demoMu.Lock()
		demoStatus[demoId].Status = "done"
		demoMu.Unlock()
	}()

	c.JSON(200, gin.H{"id": demoId})
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
