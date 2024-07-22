package pgxtest

import (
	"io"
	"strings"
)

// StringReader supports testing by passing SQL directly to the reader.
type StringReader struct{}

// Files returns the list of paths registered with the StringReader.
func (str *StringReader) Files(directory string) ([]string, error) {
	return nil, nil
}

// Read the SQL migration from the in-memory migration.
func (str *StringReader) Read(sql string) (io.Reader, error) {
	return strings.NewReader(sql), nil
}
