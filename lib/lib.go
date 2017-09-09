package lib

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/viper"
)

var (
	Versions []string = []string{
		/*"2.3",
		  "4.0",
		  "4.1",
		  "4.2",
		  "4.3",
		  "4.4",*/
		"5.0",
		"5.1",
		"6.0",
		"7.0",
		"7.1",
		"8.0"}
	Arches []string = []string{
		"arm",
		"arm64",
		"x86",
		"x86_64"}
)

const NOARCH string = "noarch"

func StringOrDefault(item interface{}, def string) string {
	if item != nil {
		str, ok := item.(string)
		if ok {
			return str
		}
	}
	return def
}

func BoolOrDefault(item interface{}, def bool) bool {
	if item != nil {
		b, ok := item.(bool)
		if ok {
			return b
		}
	}
	return def
}

func StringSliceOrNil(item interface{}) []string {
	if item != nil {
		var slice []string
		arr, ok := item.([]interface{})
		if ok {
			for _, val := range arr {
				str, ok := val.(string)
				if ok {
					slice = append(slice, str)
				}
			}
			return slice
		}
	}
	return nil
}

func StringSliceContains(slice []string, str string) bool {
	Debug("Searching for \"" + str + "\"")
	for {
		if slice == nil || len(slice) == 0 {
			// Nothing to search against
			return false
		}
		i := len(slice) / 2
		if slice[i] == str {
			return true
		} else if str < slice[i] {
			slice = slice[0:i]
		} else if str > slice[i] {
			slice = slice[i+1 : len(slice)]
		}
	}
}

func StringIntersection(strs1 []string, strs2 []string) []string {
	var shorter *[]string
	var longer *[]string
	if len(strs1) > len(strs2) {
		longer = &strs1
		shorter = &strs2
	} else {
		longer = &strs2
		shorter = &strs1
	}

	var results []string
	for _, str := range *shorter {
		if StringSliceContains(*longer, str) {
			results = append(results, str)
		}
	}
	return results
}

func GenerateMD5File(path string) error {
	fmt.Println("Generating MD5 file for " + path)
	text, err := GetHash(path, "md5")
	if err != nil {
		return fmt.Errorf("Error while generating the md5 for %v:\n  %v", path, err)
	}
	text = text + "  " + path[strings.LastIndex(path, string(os.PathSeparator))+1:] + "\n"
	err = ioutil.WriteFile(path+".md5", []byte(text), 0644)
	if err != nil {
		return fmt.Errorf("Error while writing the md5 file for %v:\n  %v", path, err)
	}
	return nil
}

func GetHash(fileToHash, algo string) (string, error) {
	var hash hash.Hash
	switch algo {
	case "md5":
		hash = md5.New()
	case "sha1":
		hash = sha1.New()
	case "sha256":
		hash = sha256.New()
	default:
		return "", fmt.Errorf("Unknown hash algorithm: %v", algo)
	}

	file, err := os.Open(fileToHash)
	if err != nil {
		return "", fmt.Errorf("Error while opening the file at %v for reading:\n  %v", fileToHash, err)
	}
	defer file.Close()

	_, err = io.Copy(hash, file)
	if err != nil {
		return "", fmt.Errorf("Error while reading the file at %v:\n  %v", fileToHash, err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func Debug(msg string) {
	if viper.GetBool("debug") {
		fmt.Println("DEBUG: " + msg)
	}
}
