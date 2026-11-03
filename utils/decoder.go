package utils

import (
	"encoding/base64"
	"fmt"
	"html"
)

func DecodeAdLink(encodedStr string) (string, error) {
	if encodedStr == "" {
		return "", fmt.Errorf("empty string")
	}
	reversed := reverseString(encodedStr)

	decodedBytes, err := base64.StdEncoding.DecodeString(reversed)
	if err != nil {
		return "", err
	}

	htmlUnescaped := html.UnescapeString(string(decodedBytes))

	return htmlUnescaped, nil
}

func Base64Decode(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("empty string")
	}
	decodedBytes, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return "", err
	}
	return string(decodedBytes), nil
}

// Helper function to reverse a string
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// unshuffleStringByStep reconstructs an original string from a shuffled version.
// The input was created by picking characters using a stepping cursor (advancing
// by step positions through the original, skipping already-used positions).
// This function inverts the process: it walks the same cursor sequence and places
// each shuffled[i] back into original[index].
func unshuffleStringByStep(shuffled string, step int) (string, error) {
	runes := []rune(shuffled)
	length := len(runes)
	if length == 0 {
		return "", fmt.Errorf("empty string")
	}
	if step <= 0 {
		return "", fmt.Errorf("step must be greater than 0")
	}

	original := make([]rune, length)
	used := make([]bool, length)
	index := 0

	for i := 0; i < length; i++ {
		for used[index] {
			index = (index + 1) % length
		}
		used[index] = true
		original[i] = runes[index]
		index = (index + step) % length
	}

	return string(original), nil
}

// DecodeStarckDataU decodes the obfuscated magnet link from the data-u attribute. This is indexer-specific.
func DecodeStarckDataU(dataU string) (string, error) {
	if dataU == "" {
		return "", fmt.Errorf("empty data-u value")
	}

	unshuffled, err := unshuffleStringByStep(dataU, 3)
	if err != nil {
		return "", fmt.Errorf("unshuffle failed: %w", err)
	}

	if !IsMagnetLink(unshuffled) {
		return "", fmt.Errorf("decoded string is not a valid magnet link: %s", unshuffled)
	}

	return unshuffled, nil
}

func IsMagnetLink(link string) bool {
	return len(link) > 8 && link[:8] == "magnet:?"
}

// DecodeStarckXid decodes the obfuscated magnet link from the "xid" query parameter
// of the download links (e.g. https://filmedl.com/?xid=...). The value is the base64
// of the magnet link, shuffled with the same stepping cursor used by data-u. This is indexer-specific.
func DecodeStarckXid(xid string) (string, error) {
	if xid == "" {
		return "", fmt.Errorf("empty xid value")
	}

	unshuffled, err := unshuffleStringByStep(xid, 3)
	if err != nil {
		return "", fmt.Errorf("unshuffle failed: %w", err)
	}

	var decoded []byte
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		decoded, err = enc.DecodeString(unshuffled)
		if err == nil {
			break
		}
	}
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	magnetLink := string(decoded)
	if !IsMagnetLink(magnetLink) {
		return "", fmt.Errorf("decoded string is not a valid magnet link: %s", magnetLink)
	}

	return magnetLink, nil
}
