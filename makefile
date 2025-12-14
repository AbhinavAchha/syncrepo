build:
	go build -o syncrepo ./*.go
	sudo mv syncrepo /usr/local/bin/syncrepo

install:
	sudo mv syncrepo /usr/local/bin/syncrepo

uninstall:
	sudo rm /usr/local/bin/syncrepo

clean:
	rm -f syncrepo
