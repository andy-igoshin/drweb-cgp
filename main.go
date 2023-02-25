package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"git.vsu.ru/ai/drweb-cgp/cgp"
	"git.vsu.ru/ai/drweb-cgp/drweb"
)

func main() {

	var arg, cmd string
	var seq int
	var line []byte
	var err error
	var n int

	in := bufio.NewReader(os.Stdin)

finalize:
	for {
		line, err = in.ReadSlice('\n')
		if err != nil {
			cgp.Putline("* error: %s\n", err)
			break finalize
		}

		n, err = fmt.Sscan(string(line), &seq, &cmd, &arg)
		if err != nil && err != io.EOF {
			cgp.Putline("* error: %s\n", err)
			continue
		}

		switch {
		case cmd == "FILE" && n == 3:
			go drweb.Scan(seq, arg)

		case cmd == "INTF" && n == 3:
			cgp.Intf(seq, arg)

		case cmd == "QUIT" && n == 2:
			cgp.Ok(seq)
			break finalize

		default:
			cgp.Putline("* bad command: %s\n", line)
		}
	}

}
