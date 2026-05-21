package prompt

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// readLine reads a line with arrow keys, home/end, delete, backspace support.
// prompt is printed first and used to calculate redraw positions.
// Opens /dev/tty directly so it works correctly under `curl | sudo bash`.
func readLine(prompt string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		fmt.Print(prompt)
		return readSimple()
	}
	defer tty.Close()

	fd := int(tty.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Print(prompt)
		return readSimple()
	}
	defer term.Restore(fd, oldState)

	write := func(s string) { tty.WriteString(s) }
	read := func() (byte, error) {
		var b [1]byte
		_, err := tty.Read(b[:])
		return b[0], err
	}
	refresh := func(buf []byte, pos int) {
		write("\r")
		write(prompt)
		tty.Write(buf)
		write("\033[K")
		write(fmt.Sprintf("\033[%dG", len(prompt)+pos+1))
	}
	cursor := func(col int) { write(fmt.Sprintf("\033[%dG", col)) }

	write(prompt)

	var buf []byte
	pos := 0

	for {
		b, err := read()
		if err != nil {
			return "", err
		}

		switch b {
		case '\r', '\n':
			write("\r\n")
			return string(buf), nil

		case 3: // Ctrl-C
			write("\r\n")
			return "", fmt.Errorf("interrupted")

		case 4: // Ctrl-D
			if len(buf) == 0 {
				write("\r\n")
				return "", fmt.Errorf("interrupted")
			}

		case 127, 8: // Backspace
			if pos > 0 {
				buf = append(buf[:pos-1], buf[pos:]...)
				pos--
				refresh(buf, pos)
			}

		case 27: // ESC sequence
			seq0, _ := read()
			if seq0 != '[' {
				continue
			}
			seq1, _ := read()
			switch seq1 {
			case 'D': // Left
				if pos > 0 {
					pos--
					write("\033[D")
				}
			case 'C': // Right
				if pos < len(buf) {
					pos++
					write("\033[C")
				}
			case 'H': // Home
				pos = 0
				cursor(len(prompt) + 1)
			case 'F': // End
				pos = len(buf)
				cursor(len(prompt) + len(buf) + 1)
			case '3': // Delete (ESC [ 3 ~)
				read() // consume ~
				if pos < len(buf) {
					buf = append(buf[:pos], buf[pos+1:]...)
					refresh(buf, pos)
				}
			case '1': // Home alt (ESC [ 1 ~)
				read() // consume ~
				pos = 0
				cursor(len(prompt) + 1)
			case '4': // End alt (ESC [ 4 ~)
				read() // consume ~
				pos = len(buf)
				cursor(len(prompt) + len(buf) + 1)
			}

		case 1: // Ctrl-A (Home)
			pos = 0
			cursor(len(prompt) + 1)

		case 5: // Ctrl-E (End)
			pos = len(buf)
			cursor(len(prompt) + len(buf) + 1)

		case 21: // Ctrl-U (clear to start)
			buf = buf[pos:]
			pos = 0
			refresh(buf, pos)

		case 11: // Ctrl-K (clear to end)
			buf = buf[:pos]
			refresh(buf, pos)

		default:
			if b >= 0x20 && b < 0x7F {
				buf = append(buf, 0)
				copy(buf[pos+1:], buf[pos:])
				buf[pos] = b
				pos++
				if pos == len(buf) {
					tty.Write([]byte{b})
				} else {
					refresh(buf, pos)
				}
			}
		}
	}
}
