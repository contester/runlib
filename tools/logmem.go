package tools

import (
	"runtime"
	"time"
	"log"
	"runlib/contester_proto"
	l4g "code.google.com/p/log4go"
)

func LogMem() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	l4g.Logf(l4g.INFO, "Alloc: %d, Sys: %d, HeapAlloc: %d, HeapIdle: %d, NextGC: %d, LastGC: %s, NumGC: %d, Blobs: %d",
		m.Alloc, m.Sys, m.HeapAlloc, m.HeapIdle, m.NextGC, time.Now().Sub(time.Unix(0, int64(m.LastGC))), m.NumGC, contester_proto.BlobCount())
}

func LogMemLoop() {
	for {
		LogMem()
		runtime.GC()
		runtime.Gosched()
		runtime.GC()
		time.Sleep(time.Second*15)
	}
}

type lwrapper struct {}

func (lw *lwrapper) Write(p []byte) (n int, err error) {
	l4g.Log(l4g.ERROR, "compat", string(p))
	return n, nil
}

func SetupLogWrapper() {
	lw := &lwrapper{}
	log.SetOutput(lw)
}

func SetupLogFile(name string) {
	l4g.Global.AddFilter("log", l4g.FINE, l4g.NewFileLogWriter(name, true))
}

func SetupLog(name string) {
	SetupLogFile(name)
	SetupLogWrapper()
}
