// Package i18n picks the Indonesian or English text for user-facing backend messages.
// Texts are written side by side at the call site: i18n.L("Indonesia", "English").
package i18n

import (
	"fmt"
	"sync/atomic"
)

// Supported languages.
const (
	ID = "id"
	EN = "en"
)

var current atomic.Value

func init() { current.Store(ID) }

// Normalize returns a supported language code (Indonesian by default).
func Normalize(lang string) string {
	if lang == EN {
		return EN
	}
	return ID
}

// Set changes the active language.
func Set(lang string) { current.Store(Normalize(lang)) }

// Lang returns the active language.
func Lang() string { return current.Load().(string) }

// L returns the text for the active language.
func L(id, en string) string {
	if Lang() == EN {
		return en
	}
	return id
}

// F formats the text for the active language.
func F(id, en string, args ...any) string { return fmt.Sprintf(L(id, en), args...) }
