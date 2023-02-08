package main

import (
	"syscall"
	"time"
)

func (f *File) CTime() time.Time {
	fileTime := f.Info.Sys().(*syscall.Win32FileAttributeData).CreationTime
	return time.Unix(fileTime.Nanoseconds()/int64(time.Second), 0)
}
