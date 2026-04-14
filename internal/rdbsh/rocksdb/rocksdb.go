package rocksdb

/*
#cgo darwin CFLAGS: -I/opt/homebrew/include -I/usr/local/include
#cgo linux CFLAGS: -I/usr/include -I/usr/local/include
#cgo darwin LDFLAGS: -L/opt/homebrew/lib -L/usr/local/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd
#cgo linux LDFLAGS: -L/usr/lib -L/usr/local/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd
#include <stdlib.h>
#include <rocksdb/c.h>
*/
import "C"

import (
	"errors"
	"unsafe"
)

type DB struct {
	db      *C.rocksdb_t
	opts    *C.rocksdb_options_t
	handles map[string]*ColumnFamilyHandle
	order   []*ColumnFamilyHandle
}

type ColumnFamilyHandle struct {
	name   string
	handle *C.rocksdb_column_family_handle_t
}

type ReadOptions struct {
	opts *C.rocksdb_readoptions_t
}

type WriteOptions struct {
	opts *C.rocksdb_writeoptions_t
}

type Iterator struct {
	it *C.rocksdb_iterator_t
}

func Open(path string) (*DB, error) {
	return open(path, false)
}

func OpenReadOnly(path string) (*DB, error) {
	return open(path, true)
}

func OpenWithColumnFamilies(path string, names []string, readOnly bool) (*DB, error) {
	if len(names) == 0 {
		return nil, errors.New("no column families provided")
	}

	opts := C.rocksdb_options_create()
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	cNames := make([]*C.char, len(names))
	cfOpts := make([]*C.rocksdb_options_t, len(names))
	handles := make([]*C.rocksdb_column_family_handle_t, len(names))

	for i, name := range names {
		cNames[i] = C.CString(name)
		cfOpts[i] = opts
	}
	defer freeCStringSlice(cNames)

	var cerr *C.char
	var db *C.rocksdb_t
	if readOnly {
		db = C.rocksdb_open_for_read_only_column_families(
			opts,
			cpath,
			C.int(len(names)),
			(**C.char)(unsafe.Pointer(unsafe.SliceData(cNames))),
			(**C.rocksdb_options_t)(unsafe.Pointer(unsafe.SliceData(cfOpts))),
			(**C.rocksdb_column_family_handle_t)(unsafe.Pointer(unsafe.SliceData(handles))),
			0,
			&cerr,
		)
	} else {
		db = C.rocksdb_open_column_families(
			opts,
			cpath,
			C.int(len(names)),
			(**C.char)(unsafe.Pointer(unsafe.SliceData(cNames))),
			(**C.rocksdb_options_t)(unsafe.Pointer(unsafe.SliceData(cfOpts))),
			(**C.rocksdb_column_family_handle_t)(unsafe.Pointer(unsafe.SliceData(handles))),
			&cerr,
		)
	}
	if err := rocksError(cerr); err != nil {
		for _, handle := range handles {
			if handle != nil {
				C.rocksdb_column_family_handle_destroy(handle)
			}
		}
		C.rocksdb_options_destroy(opts)
		return nil, err
	}

	handleMap := make(map[string]*ColumnFamilyHandle, len(names))
	order := make([]*ColumnFamilyHandle, 0, len(names))
	for i, name := range names {
		handle := &ColumnFamilyHandle{name: name, handle: handles[i]}
		handleMap[name] = handle
		order = append(order, handle)
	}

	return &DB{db: db, opts: opts, handles: handleMap, order: order}, nil
}

func ListColumnFamilies(path string) ([]string, error) {
	opts := C.rocksdb_options_create()
	defer C.rocksdb_options_destroy(opts)

	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	var count C.size_t
	var cerr *C.char
	list := C.rocksdb_list_column_families(opts, cpath, &count, &cerr)
	if err := rocksError(cerr); err != nil {
		return nil, err
	}
	if list == nil {
		return nil, nil
	}
	defer C.rocksdb_list_column_families_destroy(list, count)

	items := unsafe.Slice(list, int(count))
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, C.GoString(item))
	}
	return names, nil
}

func (d *DB) Close() {
	for _, handle := range d.order {
		if handle != nil && handle.handle != nil {
			C.rocksdb_column_family_handle_destroy(handle.handle)
		}
	}
	if d.db != nil {
		C.rocksdb_close(d.db)
	}
	if d.opts != nil {
		C.rocksdb_options_destroy(d.opts)
	}
	clear(d.handles)
	d.order = nil
}

func (d *DB) ColumnFamily(name string) (*ColumnFamilyHandle, bool) {
	handle, ok := d.handles[name]
	return handle, ok
}

func (d *DB) Get(ro *ReadOptions, key []byte) ([]byte, bool, error) {
	return d.get(ro, nil, key)
}

func (d *DB) GetCF(ro *ReadOptions, cf *ColumnFamilyHandle, key []byte) ([]byte, bool, error) {
	return d.get(ro, cf, key)
}

func (d *DB) Put(wo *WriteOptions, key, value []byte) error {
	return d.put(wo, nil, key, value)
}

func (d *DB) PutCF(wo *WriteOptions, cf *ColumnFamilyHandle, key, value []byte) error {
	return d.put(wo, cf, key, value)
}

func (d *DB) Delete(wo *WriteOptions, key []byte) error {
	return d.delete(wo, nil, key)
}

func (d *DB) DeleteCF(wo *WriteOptions, cf *ColumnFamilyHandle, key []byte) error {
	return d.delete(wo, cf, key)
}

func (d *DB) GetProperty(name string) string {
	return d.getProperty(nil, name)
}

func (d *DB) GetPropertyCF(cf *ColumnFamilyHandle, name string) string {
	return d.getProperty(cf, name)
}

func (d *DB) NewIterator(ro *ReadOptions) *Iterator {
	return &Iterator{it: C.rocksdb_create_iterator(d.db, ro.opts)}
}

func (d *DB) NewIteratorCF(ro *ReadOptions, cf *ColumnFamilyHandle) *Iterator {
	return &Iterator{it: C.rocksdb_create_iterator_cf(d.db, ro.opts, cf.handle)}
}

func NewDefaultReadOptions() *ReadOptions {
	return &ReadOptions{opts: C.rocksdb_readoptions_create()}
}

func (ro *ReadOptions) Destroy() {
	if ro != nil && ro.opts != nil {
		C.rocksdb_readoptions_destroy(ro.opts)
	}
}

func NewDefaultWriteOptions() *WriteOptions {
	return &WriteOptions{opts: C.rocksdb_writeoptions_create()}
}

func (wo *WriteOptions) Destroy() {
	if wo != nil && wo.opts != nil {
		C.rocksdb_writeoptions_destroy(wo.opts)
	}
}

func (it *Iterator) SeekToFirst() {
	C.rocksdb_iter_seek_to_first(it.it)
}

func (it *Iterator) Seek(target []byte) {
	C.rocksdb_iter_seek(it.it, bytePtr(target), C.size_t(len(target)))
}

func (it *Iterator) Valid() bool {
	return C.rocksdb_iter_valid(it.it) != 0
}

func (it *Iterator) Next() {
	C.rocksdb_iter_next(it.it)
}

func (it *Iterator) Key() []byte {
	var length C.size_t
	ptr := C.rocksdb_iter_key(it.it, &length)
	if ptr == nil {
		return nil
	}
	return C.GoBytes(unsafe.Pointer(ptr), C.int(length))
}

func (it *Iterator) Value() []byte {
	var length C.size_t
	ptr := C.rocksdb_iter_value(it.it, &length)
	if ptr == nil {
		return nil
	}
	return C.GoBytes(unsafe.Pointer(ptr), C.int(length))
}

func (it *Iterator) Err() error {
	var cerr *C.char
	C.rocksdb_iter_get_error(it.it, &cerr)
	return rocksError(cerr)
}

func (it *Iterator) Close() {
	if it != nil && it.it != nil {
		C.rocksdb_iter_destroy(it.it)
	}
}

func open(path string, readOnly bool) (*DB, error) {
	opts := C.rocksdb_options_create()
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	var cerr *C.char
	var db *C.rocksdb_t
	if readOnly {
		db = C.rocksdb_open_for_read_only(opts, cpath, 0, &cerr)
	} else {
		db = C.rocksdb_open(opts, cpath, &cerr)
	}
	if err := rocksError(cerr); err != nil {
		C.rocksdb_options_destroy(opts)
		return nil, err
	}

	return &DB{db: db, opts: opts}, nil
}

func (d *DB) get(ro *ReadOptions, cf *ColumnFamilyHandle, key []byte) ([]byte, bool, error) {
	var length C.size_t
	var cerr *C.char
	var value *C.char
	if cf == nil {
		value = C.rocksdb_get(d.db, ro.opts, bytePtr(key), C.size_t(len(key)), &length, &cerr)
	} else {
		value = C.rocksdb_get_cf(d.db, ro.opts, cf.handle, bytePtr(key), C.size_t(len(key)), &length, &cerr)
	}
	if err := rocksError(cerr); err != nil {
		return nil, false, err
	}
	if value == nil {
		return nil, false, nil
	}
	defer C.rocksdb_free(unsafe.Pointer(value))
	return C.GoBytes(unsafe.Pointer(value), C.int(length)), true, nil
}

func (d *DB) put(wo *WriteOptions, cf *ColumnFamilyHandle, key, value []byte) error {
	var cerr *C.char
	if cf == nil {
		C.rocksdb_put(d.db, wo.opts, bytePtr(key), C.size_t(len(key)), bytePtr(value), C.size_t(len(value)), &cerr)
	} else {
		C.rocksdb_put_cf(d.db, wo.opts, cf.handle, bytePtr(key), C.size_t(len(key)), bytePtr(value), C.size_t(len(value)), &cerr)
	}
	return rocksError(cerr)
}

func (d *DB) delete(wo *WriteOptions, cf *ColumnFamilyHandle, key []byte) error {
	var cerr *C.char
	if cf == nil {
		C.rocksdb_delete(d.db, wo.opts, bytePtr(key), C.size_t(len(key)), &cerr)
	} else {
		C.rocksdb_delete_cf(d.db, wo.opts, cf.handle, bytePtr(key), C.size_t(len(key)), &cerr)
	}
	return rocksError(cerr)
}

func (d *DB) getProperty(cf *ColumnFamilyHandle, name string) string {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	var value *C.char
	if cf == nil {
		value = C.rocksdb_property_value(d.db, cname)
	} else {
		value = C.rocksdb_property_value_cf(d.db, cf.handle, cname)
	}
	if value == nil {
		return ""
	}
	defer C.rocksdb_free(unsafe.Pointer(value))
	return C.GoString(value)
}

func rocksError(cerr *C.char) error {
	if cerr == nil {
		return nil
	}
	defer C.rocksdb_free(unsafe.Pointer(cerr))
	return errors.New(C.GoString(cerr))
}

func bytePtr(data []byte) *C.char {
	return (*C.char)(unsafe.Pointer(unsafe.SliceData(data)))
}

func freeCStringSlice(values []*C.char) {
	for _, value := range values {
		if value != nil {
			C.free(unsafe.Pointer(value))
		}
	}
}
