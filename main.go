package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strconv"

	"git.vsu.ru/ai/drweb-cgp/cgp"
	"git.vsu.ru/ai/drweb-cgp/config"
	"git.vsu.ru/ai/drweb-cgp/drweb"
	"git.vsu.ru/ai/drweb-cgp/utils"
)

func main() {

	if err := cgp.InitStdoutFd(); err != nil {
		os.Stderr.WriteString("failed to init CGP stdout\n")
		os.Exit(1)
	}

	conf, err := config.New(os.Args[1:])
	if err != nil {
		os.Stderr.WriteString("Error: " + err.Error() + "\n")
		os.Exit(1)
	}
	config.SetGlobal(conf)

	if config.ShowVersion() {
		os.Stderr.WriteString(config.GetVersion())
		return
	}

	in := bufio.NewReader(os.Stdin)

loop:
	for {
		line, err := in.ReadSlice('\n')
		if err != nil {
			if err != io.EOF {
				cgp.Putline("* stdin error: ", err)
			}
			break loop
		}

		parts := bytes.Fields(line)
		n := len(parts)
		if n < 2 {
			continue
		}

		seq, err := strconv.Atoi(utils.Bytes2string(parts[0]))
		if err != nil {
			continue
		}

		cmd := utils.Bytes2string(parts[1])

		switch {
		case cmd == "FILE" && n == 3:
			go drweb.Scan(seq, string(parts[2]))

		case cmd == "INTF" && n == 3:
			cgp.Intf(seq, utils.Bytes2string(parts[2]))

		case cmd == "QUIT" && n == 2:
			cgp.Ok(seq)
			break loop

		default:
			cgp.Putline("* bad command: ", line)
		}
	}
}
