package drweb

import (
	"fmt"

	"git.vsu.ru/ai/drweb-cgp/cgp"
	"git.vsu.ru/ai/drweb-cgp/clamd"
	"git.vsu.ru/ai/drweb-cgp/config"
)

func Scan(seq int, filename string) {

	qid, h, seen, err := cgp.Message(filename)
	defer h.Close()
	if err != nil {
		cgp.Failure(seq, qid, err)
		return
	}

	if seen {
		cgp.OkSeen(seq, qid)
		return
	}

	var r []clamd.Response

	if config.Transport() == config.TrUnix {
		r, err = clamd.Fildes(h)
	} else {
		r, err = clamd.InStream(h)
	}

	if err != nil {
		cgp.Failure(seq, qid, err)
		return
	}

	if len(r) == 0 {
		cgp.Failure(seq, qid, fmt.Errorf("incorrect response"))
		return
	}

	switch r[0].Status {
	case clamd.Ok:
		cgp.Ok(seq)

	case clamd.Found:
		cgp.Putline("* %d [%d]: %s\n", seq, qid, r[0].Raw)
		if config.MirrorDiscard() {
			cgp.MirrorTo(seq, qid, filename, config.MirrorTo())
		} else {
			cgp.Discard(seq)
		}

	case clamd.Error:
		cgp.Failure(seq, qid, err)

	default:
		cgp.Putline("* %d [%d]: %s\n", seq, qid, err)
		cgp.Failure(seq, qid, err)
	}
}
