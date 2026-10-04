package cgp

import (
	"strings"
	"testing"
)

func TestFindLongerDKIM(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{"Empty", []string{}, ""},
		{"Single", []string{"abc"}, "abc"},
		{"Longest first", []string{"verylongstring", "short", "medium"}, "verylongstring"},
		{"Longest last", []string{"short", "medium", "verylongstring"}, "verylongstring"},
		{"Same length", []string{"aaa", "bbb"}, "aaa"}, // вернет первый встречный
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := findLongerDKIM(tt.input)
			if res != tt.expected {
				t.Errorf("got %s, want %s", res, tt.expected)
			}
		})
	}
}

func TestLookupDKIM_Real(t *testing.T) {
	// Тест на реальном домене (например, gmail) для проверки сетевой связности.
	// Селекторы крупных сервисов обычно стабильны.
	query := "20230601._domainkey.google.com"
	key := lookupDKIM(query)

	if key == "" {
		t.Log("Warning: DNS lookup failed (could be network/resolver issue)")
	} else {
		if !strings.Contains(key, "p=") {
			t.Errorf("Result doesn't look like a DKIM key: %s", key)
		}
	}
}

func TestGetDKIMKey_CacheAndLogic(t *testing.T) {
	// 1. Проверка пустой строки
	if res := getDKIMKey(""); res != "" {
		t.Error("Empty query should return empty string")
	}

	// 2. Проверка нормализации (Case-insensitivity)
	// Мы не можем гарантировать наличие интернета, но можем проверить,
	// что повторный вызов с разным регистром возвращает тот же результат (из кэша).
	q1 := "Selector._DomainKey.Example.Com"
	q2 := "selector._domainkey.example.com"

	res1 := getDKIMKey(q1)
	res2 := getDKIMKey(q2)

	if res1 != res2 {
		t.Error("getDKIMKey should be case-insensitive")
	}
}

func TestLookupDKIM_Filtering(t *testing.T) {
	// Этот тест проверяет логику фильтрации записей внутри lookupDKIM.
	// Поскольку функция напрямую использует net.LookupTXT, мы тестируем
	// поведение на "мусорных" или пустых результатах.

	badQuery := "nonexistent-selector-12345.example.com"
	res := lookupDKIM(badQuery)
	if res != "" {
		t.Errorf("Expected empty string for non-existent record, got %s", res)
	}
}
