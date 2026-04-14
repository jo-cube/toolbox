package hello

import (
	"fmt"
	"io"
)

const message = "Hello, world!"

func Message() string {
	return message
}

func Print(w io.Writer) error {
	_, err := fmt.Fprintln(w, message)
	return err
}
