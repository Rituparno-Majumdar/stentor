package platforms

import (
	"bufio"
	"fmt"
	"strings"
)

// prompt prints a label, reads one line from in, and trims it.
func prompt(in *bufio.Reader, label string) (string, error) {
	fmt.Print(label)
	line, err := in.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
