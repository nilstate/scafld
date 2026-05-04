package terminal

import (
	"fmt"
	"io"
)

// WriteLine writes a formatted line to w.
func WriteLine(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, format+"\n", args...)
}
