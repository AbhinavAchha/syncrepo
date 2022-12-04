# Syncrepo
syncrepo synchronizes all the repositories in a given path

Given a path it runs `git pull --all` command in all the repositories. It uses sync.WaitGroup to runs these commands parallely.

## Installation
`
go install github.com/AbhinavAchha/syncrepo@latest
`

## Usage

### Pull All Repositories in given Path
`syncrepo --path path/to/the/directory`

### Get List of Repositories and Save it to given File
`syncrepo --list path --file repolist.txt`

### Export List of Repositories (JSON Only)
`syncrepo --export --path path --file repos.json`

### Import List if Repositories (JSON Only)
`syncrepo --import --path path/to/import --file repos.json`


##
Currently only JSON files importing and exporting are supported
