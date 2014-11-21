package main

import (
	"archive/zip"
	"log"
	"os"

	"path"
	"strings"

	"github.com/allanlw/zipserve/zipfs"
	"golang.org/x/tools/godoc/vfs"
)

type ZipOpenFS struct {
	fs vfs.FileSystem
}

func NewZipOpeningFS(fs vfs.FileSystem) FSCloser {
	return &ZipOpenFS{fs}
}

func (fs *ZipOpenFS) String() string {
	return "Zip opening fs on (" + fs.fs.String() + ")"
}

// Returns a ReadSeekCloser (with an emphasis on Close), and an error
func (fs *ZipOpenFS) recursively_open(prefix string, parts []string) (vfs.ReadSeekCloser, error) {
	if len(parts) == 0 {
		return fs.fs.Open(prefix)
	}

	fi, err := fs.fs.Stat(prefix)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		return fs.recursively_open(path.Join(prefix, parts[0]), parts[1:])
	}

	fh, err := fs.Open(prefix)
	if err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(ReadSeekerToReaderAt(fh), fi.Size())
	if err != nil {
		fh.Close()
		return nil, err
	}

	// Closing the zipfs should also close the backing file
	zfs := WrapFSCloserWithCloser(zipfs.New(zr, prefix).(FSCloser), fh)

	opened, err := zfs.Open(path.Join("/", path.Join(parts...)))
	if err != nil {
		zfs.Close()
		return nil, err
	}

	// closing the opened file should also close the zipfs
	// (which then closes the backing file)
	return WrapReaderWithCloser(opened, zfs), nil
}

func (fs *ZipOpenFS) Open(p string) (vfs.ReadSeekCloser, error) {
	log.Println("open:", p)
	rsc, err := fs.recursively_open("/", strings.Split(p, "/"))
	log.Println("open:", err)
	return rsc, err
}

func (fs *ZipOpenFS) recursively_stat(prefix string, parts []string, link bool) (os.FileInfo, error) {
	var fi os.FileInfo
	var err error
	if link {
		fi, err = fs.fs.Lstat(prefix)
	} else {
		fi, err = fs.fs.Stat(prefix)
	}

	if err != nil {
		return nil, err
	}
	// successful stat. change stats of zips to folders
	if len(parts) == 0 {
		if fs.isOpenableAsZipfs(prefix) {
			return MakeFakeDirFileInfo(fi.Name()), nil
		} else {
			return fi, nil
		}
	}

	if fi.IsDir() {
		return fs.recursively_stat(path.Join(prefix, parts[0]), parts[1:], link)
	}

	fh, err := fs.fs.Open(prefix)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	zr, err := zip.NewReader(ReadSeekerToReaderAt(fh), fi.Size())
	if err != nil {
		return nil, err
	}

	zfs := zipfs.New(zr, prefix).(FSCloser)
	defer zfs.Close()

	inner := path.Join("/", path.Join(parts...))
	if link {
		return zfs.Lstat(inner)
	} else {
		return zfs.Stat(inner)
	}
}

func (fs *ZipOpenFS) stat(p string, link bool) (os.FileInfo, error) {
	log.Println("stat:", p)
	fi, err := fs.recursively_stat("/", strings.Split(p, "/"), link)
	if err != nil {
		log.Println("stat:", err)
	} else {
		log.Println("stat:", fi.Name(), fi.Size(), fi.Mode(), fi.IsDir(), err)
	}
	return fi, err
}

func (fs *ZipOpenFS) Lstat(p string) (os.FileInfo, error) {
	return fs.stat(p, true)
}

func (fs *ZipOpenFS) Stat(p string) (os.FileInfo, error) {
	return fs.stat(p, false)
}

func (fs *ZipOpenFS) isOpenableAsZipfs(path string) bool {
	fi, err := fs.fs.Stat(path)
	if err != nil {
		return false
	}

	fh, err := fs.fs.Open(path)
	if err != nil {
		return false
	}
	defer fh.Close()

	_, err = zip.NewReader(ReadSeekerToReaderAt(fh), fi.Size())
	return err == nil
}

func (fs *ZipOpenFS) recursively_readdir(prefix string, parts []string) ([]os.FileInfo, error) {
	if len(parts) == 0 && !fs.isOpenableAsZipfs(prefix) {
		finfos, err := fs.fs.ReadDir(prefix)
		if err != nil {
			return finfos, err
		}
		for i := 0; i < len(finfos); i++ {
			if fs.isOpenableAsZipfs(path.Join(prefix, finfos[i].Name())) {
				finfos[i] = MakeFakeDirFileInfo(finfos[i].Name())
			}
		}
		return finfos, nil
	}

	fi, err := fs.fs.Stat(prefix)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		return fs.recursively_readdir(path.Join(prefix, parts[0]), parts[1:])
	}

	fh, err := fs.fs.Open(prefix)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	zr, err := zip.NewReader(ReadSeekerToReaderAt(fh), fi.Size())
	if err != nil {
		return nil, err
	}

	zfs := zipfs.New(zr, prefix).(FSCloser)
	defer zfs.Close()

	return zfs.ReadDir(path.Join("/", path.Join(parts...)))
}

func (fs *ZipOpenFS) ReadDir(p string) ([]os.FileInfo, error) {
	log.Println("readdir:", p)
	list, err := fs.recursively_readdir("/", strings.Split(p, "/"))
	log.Println("readdir:", list, err)
	return list, err
}

func (fs *ZipOpenFS) Close() error {
	switch t := fs.fs.(type) {
	case FSCloser:
		return t.Close()
	default:
		return nil
	}
}
