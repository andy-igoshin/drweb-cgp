package cgp

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"time"

	"git.vsu.ru/ai/drweb-cgp/config"
)

const (
	submitDir = "Submitted"
)

func NotifyInfected(seq int, m *Message, to string, virusName string) {
	mailid := strconv.Itoa(m.QID)
	sSeq := strconv.Itoa(seq)

	// Формируем пути: .tmp для записи, .sub для подхвата сервером CGP
	filename := submitDir + "/" + mailid + "no.sub"
	filetemp := submitDir + "/" + mailid + "no.tmp"

	err := func() error {
		fh, err := os.Create(filetemp)
		if err != nil {
			return err
		}
		defer fh.Close()

		w := bufio.NewWriterSize(fh, 8192)
		defer w.Flush()

		date := time.Now().Format(time.RFC1123Z)
		boundary := "virus-alert-" + mailid + "." + sSeq

		// 1. Конверт (Envelope)
		w.WriteString("Return-Path: <>\n")
		w.WriteString("Envelope-To: <")
		w.WriteString(to)
		w.WriteString(">\n")

		// 2. RFC822 Заголовки
		w.WriteString("From: ")
		w.WriteString(config.NotifyFrom())
		w.WriteString("\nTo: ")
		w.WriteString(to)
		w.WriteString("\nSubject: Notify Case ")
		w.WriteString(mailid)
		w.WriteString(": Virus Alert")
		w.WriteString("\nDate: ")
		w.WriteString(date)
		w.WriteString("\nMIME-Version: 1.0\nContent-Type: multipart/mixed; boundary=\"")
		w.WriteString(boundary)
		w.WriteString("\"\n\nThis is a multi-part message in MIME format.\n\n--")
		w.WriteString(boundary)

		// 3. Текстовое описание кейса
		w.WriteString("\nContent-Type: text/plain; charset=UTF-8; format=flowed\nContent-Transfer-Encoding: 8bit\n\nmail id:   ")
		w.WriteString(mailid)
		w.WriteString("\nmail from: ")
		if len(m.From) > 0 {
			w.WriteString(m.From)
		} else {
			w.WriteString("<>")
		}
		w.WriteString("\n")

		if len(m.Rcpts) > 0 {
			w.WriteString("rcpt to:   ")
			w.WriteString(m.Rcpts[0])
			w.WriteString("\n")
			for _, rcpt := range m.Rcpts[1:] {
				w.WriteString("\n           ")
				w.WriteString(rcpt)
				w.WriteString("\n")
			}
		}

		w.WriteString("date:      ")
		w.WriteString(date)
		w.WriteString("\n\ncase conditions:\n----------------\n")
		w.WriteString("The antivirus scanner detected a threat in a message sent from your address.\n")
		w.WriteString("The infected message was blocked and not delivered to the recipients.\n\n")
		w.WriteString("virus name: ")
		w.WriteString(virusName)
		w.WriteString("\n\n\n")

		// 4. Аттачмент (Оригинальные заголовки письма для идентификации)
		w.WriteString("--")
		w.WriteString(boundary)
		w.WriteString("\nContent-Disposition: attachment; filename=\"headers\"")
		w.WriteString("\nContent-Type: text/plain; charset=UTF-8\nContent-Transfer-Encoding: 8bit\n\n")

		hdrSize := m.BodyPos - m.HdrPos
		if hdrSize > 0 {
			// Читаем заголовки прямо из оригинального файла сообщения
			sr := io.NewSectionReader(m.File, m.HdrPos, hdrSize)
			if _, err = io.Copy(w, sr); err != nil {
				return err
			}
		}

		w.WriteString("\n--")
		w.WriteString(boundary)
		w.WriteString("--\n")

		if err = w.Flush(); err != nil {
			return err
		}
		return fh.Close()
	}()

	if err != nil {
		os.Remove(filetemp)
		Putline("* ", sSeq, " [", mailid, "]: notify error: ", err)
		return
	}

	if err = os.Rename(filetemp, filename); err != nil {
		os.Remove(filetemp)
		Putline("* ", sSeq, " [", mailid, "]: notify rename error: ", err)
	}
}
