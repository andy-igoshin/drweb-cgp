package drweb

import (
	"fmt"

	"git.vsu.ru/ai/drweb-cgp/cgp"
	"git.vsu.ru/ai/drweb-cgp/clamd"
	"git.vsu.ru/ai/drweb-cgp/config"
)

func Scan(seq int, filename string) {

	msg, err := cgp.NewMessage(seq, filename)
	if err != nil {
		if config.Debug() {
			cgp.SaveBadFile(seq, msg.QID, filename)
		}
		cgp.Failure(seq, 0, err)
		return
	}
	defer msg.Close()

	if msg.IsSeen() {
		cgp.OkSeen(seq, msg.QID)
		return
	}

	var r []clamd.Response

	switch config.Transport() {
	case config.TrUnix:
		r, err = clamd.Fildes(msg.File)
	case config.TrTcp:
		r, err = clamd.InStream(msg.File)
	default:
		err = fmt.Errorf("unknown transport")
	}

	if err != nil {
		handleError(seq, msg.QID, filename, err)
		return
	}

	if len(r) == 0 {
		handleError(seq, msg.QID, filename, fmt.Errorf("incorrect response"))
		return
	}

	res := r[0]
	switch res.Status {
	case clamd.Ok:
		cgp.Ok(seq)

	case clamd.Found:
		cgp.Putline("* ", seq, " [", msg.QID, "]: ", res.Raw())

		var headers []string
		actualMirror := config.MirrorTo()

		if len(actualMirror) > 0 {
			if seenHdr := msg.MakeSeen(); seenHdr != "" {
				headers = append(headers, seenHdr)
			} else {
				cgp.Putline("* ", seq, " [", msg.QID, "]: warning: MirrorTo skipped (no Seen tag)")
				actualMirror = nil
			}
		}

		cgp.MirrorTo(seq, msg, actualMirror, headers)

		handleNotifications(seq, msg, res.Signature)

	case clamd.Error:
		handleError(seq, msg.QID, filename, fmt.Errorf("%s", res.Raw()))

	default:
		handleError(seq, msg.QID, filename, fmt.Errorf("unknown response: %s", res.Raw()))
	}
}
