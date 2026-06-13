package util

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Pass is an empty placeholder for no-op
func Pass() {}

// MD5 returns the MD5 hash of the given string
func MD5(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

// PrintStruct prints a given struct in pretty format with indent
func PrintStruct(v any) {
	res, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(res))
}

func UnmarshalInterface(res any, mp map[string]any, key string) error {
	if a, ok := mp[key]; ok {
		b, err := json.Marshal(a)
		if err != nil {
			return fmt.Errorf("failed to marshal key %s: %w", key, err)
		}
		if err := json.Unmarshal(b, res); err != nil {
			return fmt.Errorf("failed to unmarshal key %s: %w", key, err)
		}
	}
	return nil
}
