package main

func (f *File) CTime() time.Time {
	timespec := f.Info.Sys().(*syscall.Stat_t).Ctim // Atim, Mtim
	return time.Unix(timespec.Sec, timespec.Nsec)
}
