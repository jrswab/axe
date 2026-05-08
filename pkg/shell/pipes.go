package shell

import (
	"io"
	"os"
)

// PipeHandler enables piping data into and out of Axe agents via standard streams.
type PipeHandler struct{}

func (h *PipeHandler) ReadFromStdin() ([]byte, error) {
	return io.ReadAll(os.Stdin)
}

func (h *PipeHandler) WriteToStdout(data []byte) error {
	_, err := os.Stdout.Write(data)
	return err
}
