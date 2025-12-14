// Package main provides main command. It is used to sync all the git repositories in a directory with their remote repositories.
// It uses the 'find' command to get all the directories in the path specified by the user.
// It then runs the 'git -C pull --all' command in each directory.
// It also provides the option to save the list of git repositories to a file.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
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
	list, err := FindGitReposParallel(path, runtime.NumCPU())
	if err != nil {
		log.Fatalf("Error in finding git repositories: %v", err)
	}

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
	var repos []string

	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip unreadable dirs instead of stopping the walk
			return fs.SkipDir
		}

		// Only interested in directories
		if !d.IsDir() {
			return nil
		}

		// If the directory itself is ".git"
		if d.Name() == ".git" {
			repoRoot := filepath.Dir(path)
			repos = append(repos, repoRoot)

			// 🚀 PRUNE: don't walk inside ".git"
			return fs.SkipDir
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Error in walking the path %s: %v", path, err)
	}

	return repos
}

// getGitRepos function gets the git repositories from the list of directories
func getGitRepos(list []string) []string {
	urls := make([]string, 0, len(list))
	var wg sync.WaitGroup
	wg.Add(len(list))

	for _, dir := range list {
		go func(dir string) {
			defer wg.Done()
			dir = strings.TrimSuffix(dir, "/.git")

			output, err := exec.Command("git", "-C", dir, "config", "--get", "remote.origin.url").Output()
			if err != nil {
				slog.Error("error in getting git repo url for dir", "dir", dir, "error", err)
				return
			}

			urls = append(urls, string(output))
		}(dir)
	}

	wg.Wait()
	return urls
}

// pullGitRepos function uses goroutines to run the 'git -C pull --all' command in parallel
func pullGitRepos(list []string) {
	var wg sync.WaitGroup
	wg.Add(len(list))
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	failedRepos := make(map[string]error, 0)
	var mu sync.Mutex

	for _, dir := range list {
		go func(dir string) {
			defer wg.Done()
			if err := runCommand(dir); err != nil {
				mu.Lock()
				failedRepos[dir] = err
				mu.Unlock()
				slog.Error("error in pulling git repo for dir", "dir", dir, "error", err)
			}
		}(dir)
	}

	select {
	case <-c:
		slog.Info("Received interrupt signal, terminating...")
		os.Exit(1)
	default:
		wg.Wait()
	}

	for dir, err := range failedRepos {
		// try again by rebasing
		slog.Info("Retrying pull with rebase for dir coz of error", "dir", dir, "error", err)
		cmd := exec.Command("git", "-C", dir, "pull", "--rebase", "--depth=1")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			slog.Error("error in pulling with rebase for dir", "dir", dir, "error", err)
		} else {
			slog.Info("Successfully pulled with rebase for dir", "dir", dir)
			mu.Lock()
			delete(failedRepos, dir)
			mu.Unlock()
		}
	}

	// try again for failed repos, try reseting git hard
	for dir, err := range failedRepos {
		slog.Info("Retrying pull with reset for dir coz of error", "dir", dir, "error", err)
		cmd := exec.Command("git", "-C", dir, "reset", "--hard")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			slog.Error("error in resetting git for dir", "dir", dir, "error", err)
			continue
		}

		if err := runCommand(dir); err != nil {
			slog.Error("error in pulling after reset for dir", "dir", dir, "error", err)
		} else {
			slog.Info("Successfully pulled after reset for dir", "dir", dir)
			mu.Lock()
			delete(failedRepos, dir)
			mu.Unlock()
		}
	}
}

// runCommand function runs the 'git -C pull --all' command in the directory specified by the 'dir' argument
func runCommand(dir string) error {
	dir = strings.TrimSuffix(dir, "/.git")
	cmd := exec.Command("git", "-C", dir, "pull", "--depth=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	slog.Info("pulling dir", "dir", dir)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error pulling %s, %s", dir, err)
	}

	slog.Info("pulled dir", "dir", dir)
	return nil
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
		log.Fatalf("error in accessing path %s: %v", path, err)
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
		log.Fatalf("Error in creating file %s: %v", fileName, err)
	}

	defer file.Close()
	var b strings.Builder
	for _, url := range list {
		b.WriteString(url)
	}

	if _, err := file.WriteString(b.String()); err != nil {
		log.Fatalf("Error in writing to file %s: %v", fileName, err)
	}

	if err := file.Close(); err != nil {
		log.Fatalf("Error in closing file %s: %v", fileName, err)
	}

	slog.Info("Saved git repository list to file", "file", fileName)
}

// getExportData function gets the data to export to JSON
func getExportData(dirs []string) map[string]string {
	var mtx sync.Mutex
	var wg sync.WaitGroup

	wg.Add(len(dirs))
	prefix := parsePath(flags.path) + "/"
	jsonData := make(map[string]string, len(dirs))

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
		log.Fatalf("Error in getting git repo url for dir %s: %v", dir, err)
	}

	return strings.TrimSpace(string(output))
}

// saveFile function saves the data to a file
func saveFile(filename string, data []byte) {
	if err := os.WriteFile(filename, data, 0o644); err != nil {
		log.Fatalf("Error in writing to file %s: %v", filename, err)
	}
}

// exportJSON function exports the data to a JSON file
func exportJSON(data map[string]string) {
	result, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("Error in marshalling JSON: %v", err)
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
func importJSON(filename string) map[string]string {
	if filename == "" {
		slog.Warn("Filename not specified. Using 'export.json' as default")
		filename = "export.json"
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Error in reading file %s: %v", filename, err)
	}

	jsonData := make(map[string]string)
	if err = json.Unmarshal(data, &jsonData); err != nil {
		log.Fatalf("Error in unmarshalling JSON %s: %v", filename, err)
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
			if err := os.MkdirAll(dir, 0o755); err != nil {
				log.Fatalf("Error in creating directory %s: %v", dir, err)
			}
			clone(dir, url)
		}(dir, url)
	}

	select {
	case <-c:
		slog.Info("Received interrupt signal, terminating...")
		os.Exit(1)
	default:
		wg.Wait()
	}
}

// clone function clones the git repository
func clone(dir, url string) {
	cmd := exec.Command("git", "clone", url, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("Error in cloning git repo %s: %v", url, err)
	}

	slog.Info("Cloned git repo", "url", url)
}
