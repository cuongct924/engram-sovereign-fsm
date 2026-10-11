package prover

import (
	"archive/tar"
	"io"
)

type ioReader = io.Reader

// newTar returns a function that appends a file to a tar stream written to w.
func newTar(w io.Writer) func(name string, data []byte) {
	tw := tar.NewWriter(w)
	return func(name string, data []byte) {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))})
		_, _ = tw.Write(data)
		_ = tw.Flush()
	}
}
