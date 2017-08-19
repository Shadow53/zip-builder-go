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
	"sync"

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

type FileInfo struct {
	Url                string
	Destination        string
	InstallRemoveFiles []string
	UpdateRemoveFiles  []string
	Hash               string
	Mode               string
	FileName           string
	MD5                string
	SHA1               string
	SHA256             string
	Mux                sync.RWMutex
}

type AndroidVersionInfo struct {
	HasArchSpecificInfo bool   // Architectures were set in config. If false, just read from Arm
	Base                string // Which Android version's config this was based on
	Arch                map[string]*FileInfo
	Mux                 sync.RWMutex
}

type AndroidVersions struct {
	Version map[string]*AndroidVersionInfo
	Mux     sync.RWMutex
}

type AppInfo struct {
	PackageName             string
	UrlIsFDroidRepo         bool
	DozeWhitelist           bool
	DozeWhitelistExceptIdle bool
	DataSaverWhitelist      bool
	AllowSystemUser         bool
	BlacklistSystemUser     bool
	Android                 AndroidVersions
	Permissions             []string
	Mux                     sync.RWMutex
}

type ZipInfo struct {
	Name               string
	Arch               string
	SdkVersion         string
	InstallRemoveFiles []string
	UpdateRemoveFiles  []string
	Apps               []string
	Files              []string
	Mux                sync.RWMutex
}

type Files struct {
	File map[string]*AndroidVersions
	Mux  sync.RWMutex
}
type Apps struct {
	App map[string]*AppInfo
	Mux sync.RWMutex
}

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
