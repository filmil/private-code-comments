package main

import (
	"flag"
	"testing"
)

var (
	pidFile = flag.String("pid-file", "", "")
	dbFile  = flag.String("db-file", "", "")
)

func TestOne(t *testing.T) {
	if *pidFile == "" {
		t.Fatalf("no --pid-file flag")
	}
	if *dbFile == "" {
		t.Fatalf("no --pid-file flag")
	}
}
