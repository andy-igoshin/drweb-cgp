package cgp

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/maypok86/otter"
)

// getDKIMKey — кэширующий резолвер для TXT-записей DKIM.
// query ожидается в формате "selector._domainkey.domain.tld"
var getDKIMKey = func() func(query string) (key string) {
	cache, _ := otter.MustBuilder[string, string](2000).
		CollectStats().
		Cost(func(key string, value string) uint32 {
			return 1
		}).
		WithVariableTTL().
		Build()

	return func(query string) (key string) {
		query = strings.ToLower(strings.TrimSpace(query))
		if query == "" {
			return ""
		}

		var ok bool
		key, ok = cache.Get(query)
		if !ok {
			key = lookupDKIM(query)
			if len(key) > 0 {
				cache.Set(query, key, time.Hour)
			} else {
				cache.Set(query, key, 5*time.Minute)
			}
		}

		return
	}
}()

// lookupDKIM выполняет прямой запрос TXT-записи в DNS
func lookupDKIM(query string) string {
	r := &net.Resolver{PreferGo: true}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	txts, err := r.LookupTXT(ctx, query)
	if err != nil {
		return ""
	}

	var validRecords []string
	for _, txt := range txts {
		// Некоторые серверы отдают части одной записи как разные строки в слайсе,
		// склеиваем их, если в этом есть смысл, или фильтруем.
		if strings.Contains(txt, "v=DKIM1") || strings.Contains(txt, "p=") {
			validRecords = append(validRecords, txt)
		}
	}

	if len(validRecords) == 0 {
		return ""
	}

	if len(validRecords) > 1 {
		return findLongerDKIM(validRecords)
	}

	return validRecords[0]
}

func findLongerDKIM(items []string) string {
	if len(items) == 0 {
		return ""
	}
	var maxlen = 0
	var pos int
	for p, item := range items {
		if len(item) > maxlen {
			maxlen = len(item)
			pos = p
		}
	}
	return items[pos]
}
