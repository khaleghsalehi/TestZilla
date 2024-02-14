default:
	build
clean:
	rm testzilla
build:
	clear
	go build .
	ls -hal testzilla