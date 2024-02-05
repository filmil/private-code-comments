package main

import (
	"errors"
	"flag"
	"io/fs"
	"os"
	"syscall"

	"github.com/golang/glog"
)

func main() {
	defer func() {
		glog.Flush()
	}()
	var filename string
	flag.StringVar(
		&filename, "file", "",
		"The file name for the private comments database")

	flag.Parse()

	if filename == "" {
		glog.Fatalf("flag --file=... is required")
	}
	if err := os.Remove(filename); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			glog.Fatalf("could not remove file: %v", err)
		}
		// If the file does not exist, we're done here.
	}
	glog.V(1).Infof("creating FIFO at: %q", filename)

	if err := syscall.Mkfifo(filename, 0666); err != nil {
		glog.Fatalf("could not make FIFO at: %v: %v", filename, err)
	}
	glog.V(1).Infof("created  FIFO at: %q", filename)
}
