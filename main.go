// Package main provides main command. It is used to sync all the git repositories in a directory with their remote repositories.
// It uses the 'find' command to get all the directories in the path specified by the user.
// It then runs the 'git -C pull --all' command in each directory.
// It uses goroutines to run the commands in parallel.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

func main() {
	path := getPathFromUser()
	list, err := getDirectories(path)
	if err != nil {
		panic(err)
	}
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

// getPathFromUser function gets the 'absolute' path from the user and returns it as a string
// It also checks if the path is valid
func getPathFromUser() string {
	fmt.Print("Enter the absolute path: ")
	var path string
	if _, err := fmt.Scanln(&path); err != nil {
		panic(err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Default().Print("Path does not exist")
		log.Default().Print("Please enter a valid path")
		return getPathFromUser()
	}
	return path
}

// getDirectories function uses the 'path' argument to get all the directories in the path.
// It returns a list of directories as a string slice
func getDirectories(path string) ([]string, error) {
	output, err := exec.Command("find", path, "-name", ".git").Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(string(output), "\n"), nil
}

// runCommand function runs the 'git -C pull --all' command in the directory specified by the 'dir' argument
func runCommand(dir string) error {
	dir = strings.TrimSuffix(dir, "/.git")
	cmd := exec.Command("git", "-C", dir, "pull", "--all")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
