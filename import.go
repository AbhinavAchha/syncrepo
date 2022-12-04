package main

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"sync"
)

func importJSON(filename string) (jsonData map[string]string) {
	if filename == "" {
		log.Default().Println("No filename specified, using 'export.json'")
		filename = "export.json"
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal("Error reading file: ", err)
	}
	if err = json.Unmarshal(data, &jsonData); err != nil {
		log.Fatal(err)
	}
	return jsonData
}

func createRepos(data map[string]string) {
	wg := sync.WaitGroup{}
	wg.Add(len(data))
	importPath := parsePath(flags.path)
	for dir, url := range data {
		go func(dir, url string) {
			defer wg.Done()

			if err := os.MkdirAll(importPath+"/"+dir, 0755); err != nil {
				log.Fatal(err)
			}
			clone(dir, url)
		}(dir, url)

	}
	wg.Wait()
}

func clone(dir, url string) {
	cmd := exec.Command("git", "clone", url, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
