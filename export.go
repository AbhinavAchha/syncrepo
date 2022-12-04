package main

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// getExportData function gets the data to export to JSON
func getExportData(dirs []string) (jsonData map[string]string) {
	jsonData = make(map[string]string, len(dirs))
	wg := sync.WaitGroup{}
	wg.Add(len(dirs))
	mtx := sync.Mutex{}
	prefix := parsePath(flags.path) + "/"

	for _, dir := range dirs {
		go func(dir string) {
			defer wg.Done()
			data := getGitRepo(dir)
			dir = strings.TrimPrefix(strings.TrimSuffix(dir, "/.git"), prefix)

			mtx.Lock()
			jsonData[dir] = data
			mtx.Unlock()
		}(dir)
	}
	wg.Wait()
	return jsonData
}

// // getGitRepo function gets the git repository from the directory
func getGitRepo(dir string) string {
	output, err := exec.Command("git", "-C", dir, "config", "--get", "remote.origin.url").Output()
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimSpace(string(output))
}

// saveFile function saves the data to a file
func saveFile(filename string, data []byte) {
	if err := os.WriteFile(filename, data, 0644); err != nil {
		log.Fatal(err)
	}
}

// exportJSON function exports the data to a JSON file
func exportJSON(data map[string]string) {
	result, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fileName := flags.fileName
	if fileName == "" {
		fileName = "export"
	} else if !strings.HasSuffix(fileName, ".json") {
		fileName += ".json"
	}
	saveFile(fileName, result)
}
