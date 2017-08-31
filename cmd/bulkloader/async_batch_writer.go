package main

import (
	"sync/atomic"

	"github.com/dgraph-io/badger"
	"github.com/dgraph-io/dgraph/x"
)

type KVWriter struct {
	kv    *badger.KV
	batch []*badger.Entry
	prog  *progress
	sem   chan struct{}
}

func NewKVWriter(kv *badger.KV, prog *progress) *KVWriter {
	w := &KVWriter{
		kv:    kv,
		batch: make([]*badger.Entry, 0, writeBatchSize),
		prog:  prog,
		sem:   make(chan struct{}, 1000),
	}
	return w
}

func (w *KVWriter) Set(k, v []byte, meta byte) {
	e := &badger.Entry{
		Key:      k,
		Value:    v,
		UserMeta: meta,
	}
	w.batch = append(w.batch, e)
	if len(w.batch) == writeBatchSize {
		w.setEntries(w.batch)
		w.batch = make([]*badger.Entry, 0, writeBatchSize)
	}
}

func (w *KVWriter) Wait() {
	atomic.AddInt64(&w.prog.outstandingWrites, 1)
	err := w.kv.BatchSet(w.batch)
	checkErrs(err, w.batch)
	atomic.AddInt64(&w.prog.outstandingWrites, -1)
}

func (w *KVWriter) setEntries(entries []*badger.Entry) {
	w.sem <- struct{}{}
	atomic.AddInt64(&w.prog.outstandingWrites, 1)
	w.kv.BatchSetAsync(entries, func(err error) {
		<-w.sem
		checkErrs(err, entries)
		atomic.AddInt64(&w.prog.outstandingWrites, -1)
	})
}

func checkErrs(err error, entries []*badger.Entry) {
	x.Check(err)
	for _, e := range entries {
		x.Check(e.Error)
	}
}