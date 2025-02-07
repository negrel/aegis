package main

import (
	"bufio"
	"io"
)

// printLines reads lines of the provided reader and prints them to the given
// writer.
func printLines(r io.Reader, w io.Writer, prefix string) {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		io.WriteString(w, prefix)
		w.Write(scanner.Bytes())
		io.WriteString(w, "\n")
	}
}
