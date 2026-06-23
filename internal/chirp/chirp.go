package chirp

import (
	"strings"
)

func ValidateLength(chirp string) bool {
	return len(chirp) <= 140
}

// function to replace "profane" words with a 4-character string "****"
func RedactProfaneWords(text string) string {
	banned := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}

	words := strings.Split(text, " ")

	for i, word := range words {
		if _, exists := banned[strings.ToLower(word)]; exists {
			words[i] = "****"
		}
	}

	redacted := strings.Join(words, " ")

	return redacted
}
