package main

import (
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/tools/godoc/vfs"
)

// ReadSeekerAt is a simple wrapper around a ReadSeeker to make it
// a ReaderAt. Note that using this canibalizes the seeking behavior of
// the underlying io.ReadSeeker, so it should not be used again
type ReadSeekerAt struct {
	wrapped io.ReadSeeker
	ratLock sync.Mutex
}

func (f *ReadSeekerAt) ReadAt(p []byte, off int64) (n int, err error) {
	f.ratLock.Lock()
	defer f.ratLock.Unlock()

	_, err = f.wrapped.Seek(off, 0)
	if err != nil {
		return 0, err
	}

	n, err = f.wrapped.Read(p)
	return
}

// ReadSeekerToReaderAt takes a io.ReadSeeker and returns a new
// io.ReaderAt that will be a proxy for the ReadSeeker.
// Note that this new ReaderAt is not as powerful, because it does
// not allow ReadAt calls to be run in parallel (they lock while waiting)
func ReadSeekerToReaderAt(rs io.ReadSeeker) io.ReaderAt {
	return &ReadSeekerAt{wrapped: rs}
}

// Wraps a vfs.ReadSeekCloser by adding an extra close method
type wrappedReadSeekCloser struct {
	wrapped     vfs.ReadSeekCloser
	after_close io.Closer
}

func (wrsc *wrappedReadSeekCloser) Close() error {
	err := wrsc.wrapped.Close()
	err2 := wrsc.after_close.Close()
	if err != nil {
		return err
	} else {
		return err2
	}
}

func (wrsc *wrappedReadSeekCloser) Read(p []byte) (int, error) {
	return wrsc.wrapped.Read(p)
}

func (wrsc *wrappedReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	return wrsc.wrapped.Seek(offset, whence)
}

func WrapReaderWithCloser(rsc vfs.ReadSeekCloser, closer io.Closer) vfs.ReadSeekCloser {
	return &wrappedReadSeekCloser{rsc, closer}
}

type FSCloser interface {
	vfs.FileSystem
	io.Closer
}

// wraps a vfs.FileSystem by adding an extra close to call
type wrappedFileSystem struct {
	wrapped     FSCloser
	after_close io.Closer
}

func (wfs *wrappedFileSystem) Open(p string) (vfs.ReadSeekCloser, error) {
	return wfs.wrapped.Open(p)
}

func (wfs *wrappedFileSystem) Lstat(p string) (os.FileInfo, error) {
	return wfs.wrapped.Lstat(p)
}

func (wfs *wrappedFileSystem) Stat(p string) (os.FileInfo, error) {
	return wfs.wrapped.Stat(p)
}

func (wfs *wrappedFileSystem) ReadDir(p string) ([]os.FileInfo, error) {
	return wfs.wrapped.ReadDir(p)
}

func (wfs *wrappedFileSystem) Close() error {
	err := wfs.wrapped.Close()
	err2 := wfs.after_close.Close()
	if err != nil {
		return err
	} else {
		return err2
	}
}

func (wfs *wrappedFileSystem) String() string {
	return wfs.wrapped.String()
}

// Takes an FSCloser and returns a new FSCloser with an additional
// Close method
func WrapFSCloserWithCloser(fs FSCloser, closer io.Closer) FSCloser {
	return &wrappedFileSystem{fs, closer}
}

// FakeDirFileInfo is just a os.FileInfo that has nothing but a name
// and says it's a directory
type FakeDirFileInfo struct {
	name string
}

func (i FakeDirFileInfo) Name() string       { return i.name }
func (i FakeDirFileInfo) Size() int64        { return 0 }
func (i FakeDirFileInfo) ModTime() time.Time { return time.Time{} }
func (i FakeDirFileInfo) Mode() os.FileMode  { return os.ModeDir | 0555 }
func (i FakeDirFileInfo) IsDir() bool        { return true }
func (i FakeDirFileInfo) Sys() interface{}   { return nil }

func MakeFakeDirFileInfo(name string) os.FileInfo {
	return FakeDirFileInfo{name}
}
