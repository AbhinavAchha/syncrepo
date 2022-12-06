// Package main provides main command. It is used to sync all the git repositories in a directory with their remote repositories.
// It uses the 'find' command to get all the directories in the path specified by the user.
// It then runs the 'git -C pull --all' command in each directory.
// It also provides the option to save the list of git repositories to a file.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

var flags struct {
	path     string
	fileName string
	pull     bool
	list     bool
	help     bool
	export   bool
	toImport bool
}

func main() {
	flag.StringVar(&flags.path, "path", ".", "Path to the directory containing git repositories")
	flag.StringVar(&flags.fileName, "file", "", "File name to save the list of git repositories")
	flag.BoolVar(&flags.pull, "pull", false, "Pull all the git repositories")
	flag.BoolVar(&flags.list, "list", false, "List all the git repositories")
	flag.BoolVar(&flags.help, "help", false, "Show help")
	flag.BoolVar(&flags.export, "export", false, "Export all the git repositories to a JSON file")
	flag.BoolVar(&flags.toImport, "import", false, "Import all the git repositories from a JSON file")
	flag.Parse()

	// check if no arguments are specified
	if flags.help || len(os.Args) == 1 {
		flag.Usage()
		return
	}

	path := parsePath(flags.path)
	list := getDirectories(path)
	if flags.list {
		urls := getGitRepos(list)
		if flags.fileName != "" {
			saveToFile(flags.fileName, urls)
		} else {
			printList(urls)
		}
	}

	if flags.pull {
		pullGitRepos(list)
	}

	if flags.export {
		repoData := getExportData(list)
		exportJSON(repoData)
	}

	if flags.toImport {
		jsonData := importJSON(flags.fileName)
		createRepos(jsonData)
	}
}

// getDirectories function uses the 'path' argument to get all the directories in the path.
// It returns a list of directories as a string slice
func getDirectories(path string) []string {
	output, err := exec.Command("find", path, "-type", "d", "-name", ".git", "-not", "-path", "*/.git/modules/*").
		Output()
	if err != nil {
		log.Default().Print(err)
	}
	return strings.Split(string(output), "\n")
}

// getGitRepos function gets the git repositories from the list of directories
func getGitRepos(list []string) (urls []string) {
	urls = make([]string, 0, len(list))
	wg := sync.WaitGroup{}
	wg.Add(len(list))
	for _, dir := range list {
		go func(dir string) {
			defer wg.Done()
			dir = strings.TrimSuffix(dir, "/.git")
			cmd, err := exec.Command("git", "-C", dir, "config", "--get", "remote.origin.url").Output()
			if err != nil {
				log.Default().Print(err)
				return
			}
			urls = append(urls, string(cmd))
		}(dir)
	}
	wg.Wait()
	return urls
}

// pullGitRepos function uses goroutines to run the 'git -C pull --all' command in parallel
func pullGitRepos(list []string) {
	wg := sync.WaitGroup{}
	wg.Add(len(list))
	for _, dir := range list {
		go func(dir string) {
			defer wg.Done()
			if err := runCommand(dir); err != nil {
				log.Default().Print(err)
			}
		}(dir)
	}
	wg.Wait()
}

// runCommand function runs the 'git -C pull --all' command in the directory specified by the 'dir' argument
func runCommand(dir string) error {
	dir = strings.TrimSuffix(dir, "/.git")
	cmd := exec.Command("git", "-C", dir, "pull", "--all")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// parsePath function parses the path argument and returns the path as a string
// It also checks if the path is valid
func parsePath(path string) string {
	if path == "." {
		path, _ = os.Getwd()
		return path
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = strings.Replace(path, "~/", home, 1)
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			log.Fatal("Path does not exists. Please specify a valid path")
		}
		log.Fatal(err)
	}
	return path
}

// printList function prints the list of git repositories. It is used when the user does not specify a file name.
func printList(list []string) {
	for _, url := range list {
		log.Default().Print(url)
	}
}

// saveToFile function saves the list of git repositories to a file. The file name is specified by the 'file' argument
func saveToFile(fileName string, list []string) {
	file, err := os.Create(fileName)
	if err != nil {
		log.Default().Print(err)
		return
	}
	defer file.Close()
	for _, url := range list {
		file.WriteString(url)
	}
}

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
			if dir == "" {
				return
			}
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
		fileName = "export.json"
	} else if !strings.HasSuffix(fileName, ".json") {
		fileName += ".json"
	}
	saveFile(fileName, result)
}

// importJSON function imports the data from a JSON file
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

// createRepos function creates the git repositories
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

// clone function clones the git repository
func clone(dir, url string) {
	cmd := exec.Command("git", "clone", url, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
