package helpers

import (
	// Go Internal Packages
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"

	// External Packages
	"github.com/gorilla/schema"
)

// Pass is an empty placeholder for no-op
func Pass() {
	// do nothing
}

// MD5 returns the MD5 hash of the given string
func MD5(text string) string {
	hasher := md5.New()
	if _, err := io.WriteString(hasher, text); err != nil {
		panic(err)
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

// EscapeSpecialChars replaces special characters in a string with "\\" + the character
func EscapeSpecialChars(input string) string {
	re := regexp.MustCompile(`\W`)
	return re.ReplaceAllString(input, "\\$0")
}

// ReplaceWhitespaceWithPipe replaces whitespace with a pipe character
func ReplaceWhitespaceWithPipe(text string) string {
	re := regexp.MustCompile(`\\ `)
	return re.ReplaceAllString(text, "|")
}

// PrintStruct prints a given struct in pretty format with indent
func PrintStruct(v any) {
	res, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(res))
}

// Map applies a function to each item in a slice and returns a new slice
func Map[A any, B any](arr []A, f func(A) B) []B {
	result := make([]B, len(arr))
	for i, v := range arr {
		result[i] = f(v)
	}
	return result
}

// GetSchemaDecoder returns a new instance of schema.Decoder
func GetSchemaDecoder() *schema.Decoder {
	d := schema.NewDecoder()
	d.IgnoreUnknownKeys(true)
	return d
}

func UnmarshalInterface(res interface{}, mp map[string]interface{}, key string) error {
	if a, ok := mp[key]; ok {
		Bytes, _ := json.Marshal(a)
		err := json.Unmarshal(Bytes, res)
		if err != nil {
			return fmt.Errorf("error occurred when unmarshalling key %s: %v", key, err)
		}
	}
	return nil
}
