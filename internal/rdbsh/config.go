package rdbsh

import "io"

type Config struct {
	DBPath       string
	Writable     bool
	ColumnFamily string
	ExecCommand  string
	Force        bool
	In           io.Reader
	Out          io.Writer
	ErrOut       io.Writer
}
