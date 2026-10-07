package drweb

import (
	"errors"
	"strings"

	"git.vsu.ru/ai/drweb-cgp/cgp"
	"git.vsu.ru/ai/drweb-cgp/config"
)

func handleNotifications(seq int, msg *cgp.Message, virusName string) {
	var recipient string

	if msg.Auth != "" {
		// Аутентифицированный пользователь
		if config.IsAuthNotifyEnabled() {
			recipient = msg.From
		}
	} else if config.IsExternalNotifyEnabled() {
		// Внешний пользователь: проверка DKIM + Alignment
		dkimDomain, err := msg.VerifyDKIM()
		if err != nil {
			// Технические сбои (seek/парсинг/DNS) логируем всегда,
			// ожидаемые отрицательные исходы — только в debug.
			if errors.Is(err, cgp.ErrDKIMNoSignature) ||
				errors.Is(err, cgp.ErrDKIMInvalid) {
				if config.Debug() {
					cgp.Putline("* ", seq, " [", msg.QID, "]: ", err)
				}
			} else {
				cgp.Putline("* ", seq, " [", msg.QID, "]: dkim verify error: ", err)
			}
		}
		if dkimDomain != "" {
			fromDomain := extractDomain(msg.From)
			// Проверяем соответствие домена подписи и домена в MAIL FROM
			if strings.EqualFold(dkimDomain, fromDomain) {
				// Проверяем списки Allow/Deny из конфига
				if config.IsDomainAllowed(dkimDomain) {
					recipient = msg.From
				}
			} else {
				cgp.Putline("* ", seq, " [", msg.QID, "]: notify skipped: DKIM alignment failed (",
					dkimDomain, " != ", fromDomain, ")")
			}
		}
	}

	if recipient != "" {
		cgp.NotifyInfected(seq, msg, recipient, virusName)
	}
}

// extractDomain извлекает часть после @ из email адреса.
func extractDomain(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at == -1 || at == len(email)-1 {
		return ""
	}
	return email[at+1:]
}

func handleError(seq, qid int, filename string, err error) {
	if config.Debug() {
		cgp.SaveBadFile(seq, qid, filename)
	}
	cgp.Failure(seq, qid, err)
}
