default:
	build
clean:
	rm testzilla
	rm agent
build:
	clear
	go build -buildvcs=false -o testzilla .
	go build -buildvcs=false -o agent .
	ls -hal testzilla