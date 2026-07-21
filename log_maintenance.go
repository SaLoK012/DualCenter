package main

import (
	"os"
	"sync"
)

const maxLogSize int64 = 1 << 20 // 1 MiB por arquivo.

var logMu sync.Mutex

func rotateLogIfNeeded(path string, limit int64) {
	if path == "" || limit <= 0 {
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() < limit {
		return
	}
	backup := path + ".old"
	_ = os.Remove(backup)
	_ = os.Rename(path, backup)
}
