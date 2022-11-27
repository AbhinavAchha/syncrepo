# syncrepo
syncrepo synchronizes all the repositories in a given path

Given a path it runs `git pull --all` command in all the repositories. It uses sync.WaitGroup to runs these commands parallely.
