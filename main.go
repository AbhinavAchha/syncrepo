// Package main provides main command. It is used to sync all the git repositories in a directory with their remote repositories.
// It uses the 'find' command to get all the directories in the path specified by the user.
// It then runs the 'git -C pull --all' command in each directory.
// It also provides the option to save the list of git repositories to a file.
package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

func main() {
	helpArg := flag.Bool("help", false, "Show help")
	pathArg := flag.String(
		"path",
		".",
		"Path to the directory containing the git repositories.\nDefault is the current directory",
	)
	fileNameArg := flag.String("file", "", "File name to save the list of git repositories")
	pullArg := flag.Bool("pull", false, "Pull all the git repositories in the path")
	listArg := flag.Bool("list", false, "List all the git repositories in the path")
	flag.Parse()

	if *helpArg {
		flag.PrintDefaults()
		os.Exit(0)
	}

	path := parsePath(*pathArg)
	list := getDirectories(path)
	if *listArg {
		urls := getGitRepos(list)
		if *fileNameArg != "" {
			saveToFile(*fileNameArg, urls)
		} else {
			printList(urls)
		}
	}

	if *pullArg {
		pullGitRepos(list)
	}
}

// getDirectories function uses the 'path' argument to get all the directories in the path.
// It returns a list of directories as a string slice
func getDirectories(path string) []string {
	output, err := exec.Command("find", path, "-name", ".git").Output()
	if err != nil {
		panic(err)
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
			cmd, err := exec.Command("git", "-C", dir, "remote", "get-url", "--push", "origin").Output()
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
	} else if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		path = strings.Replace(path, "~", home, 1)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Default().Print("Path does not exist")
		log.Default().Print("Please enter a valid path")
		os.Exit(1)
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
