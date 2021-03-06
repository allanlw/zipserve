package main

import (
	"log"
	"net/http"
	"os"

	"golang.org/x/tools/godoc/vfs"
	"golang.org/x/tools/godoc/vfs/httpfs"
	"github.com/allanlw/zipserve/zipopenfs"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Supply one argument, path to serve")
	}
	serve := os.Args[1]

	fs := zipopenfs.NewZipOpeningFS(vfs.OS(serve))
	hfs := httpfs.New(fs)
	fileserver := http.FileServer(hfs)
	log.Fatal(http.ListenAndServe(":8080", fileserver))
}
