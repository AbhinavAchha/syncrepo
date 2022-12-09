// Package main provides main command. It is used to sync all the git repositories in a directory with their remote repositories.
// It uses the 'find' command to get all the directories in the path specified by the user.
// It then runs the 'git -C pull --all' command in each directory.
// It also provides the option to save the list of git repositories to a file.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
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
		os.Exit(0)
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
		os.Exit(0)
	}

	if flags.export {
		repoData := getExportData(list)
		exportJSON(repoData)
		os.Exit(0)
	}

	if flags.toImport {
		jsonData := importJSON(flags.fileName)
		createRepos(jsonData)
		os.Exit(0)
	}
}

// getDirectories function uses the 'path' argument to get all the directories in the path.
// It returns a list of directories as a string slice
func getDirectories(path string) []string {
	output, err := exec.Command("find", path, "-type", "d", "-name", ".git", "-not", "-path", "*/.git/modules/*").
		Output()
	if err != nil {
		log.Printf("Error in getting directories: %s", err)
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
				log.Printf("Error in getting git repo url %s, %s", dir, err)
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
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	for _, dir := range list {
		go func(dir string) {
			defer wg.Done()
			if err := runCommand(dir, c); err != nil {
				log.Print(err)
			}
		}(dir)
	}
	wg.Wait()
}

// runCommand function runs the 'git -C pull --all' command in the directory specified by the 'dir' argument
func runCommand(dir string, c chan os.Signal) (err error) {
	dir = strings.TrimSuffix(dir, "/.git")
	cmd := exec.Command("git", "-C", dir, "pull", "--all")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatalf("Error starting pulling process %s, %s", dir, err)
	}
	select {
	case <-c:
		log.Fatal(cmd.Process.Kill())
	default:
		err = cmd.Wait()
	}
	return err
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
		log.Fatalf("Error in getting path: %s", err)
	}
	return path
}

// printList function prints the list of git repositories. It is used when the user does not specify a file name.
func printList(list []string) {
	for _, url := range list {
		fmt.Println(url)
	}
}

// saveToFile function saves the list of git repositories to a file. The file name is specified by the 'file' argument
func saveToFile(fileName string, list []string) {
	file, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("Error in creating file: %s", err)
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
		log.Fatalf("Error getting git repo from %s: %s", dir, err)
	}
	return strings.TrimSpace(string(output))
}

// saveFile function saves the data to a file
func saveFile(filename string, data []byte) {
	if err := os.WriteFile(filename, data, 0644); err != nil {
		log.Fatalf("Error saving file: %s, %s", filename, err)
	}
}

// exportJSON function exports the data to a JSON file
func exportJSON(data map[string]string) {
	result, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("Error marshalling JSON: %s", err)
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
		log.Println("No filename specified, using 'export.json'")
		filename = "export.json"
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Error reading file: %s, %s", filename, err)
	}
	if err = json.Unmarshal(data, &jsonData); err != nil {
		log.Fatalf("Error unmarshalling JSON: %s", err)
	}
	return jsonData
}

// createRepos function creates the git repositories
func createRepos(data map[string]string) {
	wg := sync.WaitGroup{}
	wg.Add(len(data))
	importPath := parsePath(flags.path) + "/"
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	for dir, url := range data {
		go func(dir, url string) {
			defer wg.Done()
			dir = importPath + dir
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Fatalf("Error creating directory: %s, %s", dir, err)
			}
			clone(dir, url, c)
		}(dir, url)

	}
	wg.Wait()
}

// clone function clones the git repository
func clone(dir, url string, c chan os.Signal) {
	cmd := exec.Command("git", "clone", url, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatalf("Error starting clone process %s, %s", url, err)
	}
	select {
	case <-c:
		log.Fatal(cmd.Process.Kill())
	default:
		cmd.Wait()
	}
}
