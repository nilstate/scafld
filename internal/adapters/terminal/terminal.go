package terminal

import (
	"fmt"
	"io"
)

func WriteLine(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, format+"\n", args...)
}
