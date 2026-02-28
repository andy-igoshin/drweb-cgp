package drweb

import (
	"fmt"

	"git.vsu.ru/ai/drweb-cgp/cgp"
	"git.vsu.ru/ai/drweb-cgp/clamd"
	"git.vsu.ru/ai/drweb-cgp/config"
)

func Scan(seq int, filename string) {
	qid, h, seen, err := cgp.Message(filename)
	if err != nil {
		if config.Debug() {
			cgp.SaveBadFile(seq, qid, filename)
		}
		cgp.Failure(seq, qid, err)
		return
	}
	defer h.Close()

	if seen {
		cgp.OkSeen(seq, qid)
		return
	}

	var r []clamd.Response

	switch config.Transport() {
	case config.TrUnix:
		r, err = clamd.Fildes(h)
	case config.TrTcp:
		r, err = clamd.InStream(h)
	default:
		err = fmt.Errorf("unknown transport")
	}

	if err != nil {
		handleError(seq, qid, filename, err)
		return
	}

	if len(r) == 0 {
		handleError(seq, qid, filename, fmt.Errorf("incorrect response"))
		return
	}

	res := r[0]
	switch res.Status {
	case clamd.Ok:
		cgp.Ok(seq)

	case clamd.Found:
		cgp.Putline("* ", seq, " [", qid, "]: ", res.Raw())
		if config.MirrorDiscard() {
			cgp.MirrorTo(seq, qid, filename, config.MirrorTo())
		} else {
			cgp.Discard(seq)
		}

	case clamd.Error:
		handleError(seq, qid, filename, fmt.Errorf("%s", res.Raw()))

	default:
		handleError(seq, qid, filename, fmt.Errorf("unknown response: %s", res.Raw()))
	}
}

func handleError(seq, qid int, filename string, err error) {
	if config.Debug() {
		cgp.SaveBadFile(seq, qid, filename)
	}
	cgp.Failure(seq, qid, err)
}
