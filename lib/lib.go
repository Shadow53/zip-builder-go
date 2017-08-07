package lib

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"io/ioutil"
	"log"
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
}

type AndroidVersionInfo struct {
	HasArchSpecificInfo bool   // Architectures were set in config. If false, just read from Arm
	Base                string // Which Android version's config this was based on
	Arch                map[string]FileInfo
}

type AppInfo struct {
	PackageName             string
	UrlIsFDroidRepo         bool
	DozeWhitelist           bool
	DozeWhitelistExceptIdle bool
	DataSaverWhitelist      bool
	AllowSystemUser         bool
	BlacklistSystemUser     bool
	AndroidVersion          map[string]AndroidVersionInfo
	Permissions             []string
}

type ZipInfo struct {
	Name               string
	Arch               string
	SdkVersion         string
	InstallRemoveFiles []string
	UpdateRemoveFiles  []string
	Apps               []string
	Files              []string
}

type Files map[string]map[string]AndroidVersionInfo
type Apps map[string]AppInfo

func ExitIfError(err error) {
	if err != nil {
		log.Fatalln(err) // If arches specified
	}
}

func StringOrDefault(item interface{}, def string) string {
	if item != nil {
		return item.(string)
	} else {
		return def
	}
}

func BoolOrDefault(item interface{}, def bool) bool {
	if item != nil {
		return item.(bool)
	} else {
		return def
	}
}

func StringSliceOrNil(item interface{}) []string {
	if item != nil {
		var slice []string
		for _, val := range item.([]interface{}) {
			slice = append(slice, val.(string))
		}
		return slice
	} else {
		return nil
	}
}

func GenerateMD5File(path string) {
	log.Println("Generating MD5 file for " + path)
	text := GetHash(path, "md5") + "  " + path[strings.LastIndex(path, string(os.PathSeparator))+1:] + "\n"
	ioutil.WriteFile(path+".md5", []byte(text), 0644)
}

func GetHash(fileToHash, algo string) string {
	var hash hash.Hash
	switch algo {
	case "md5":
		hash = md5.New()
	case "sha1":
		hash = sha1.New()
	case "sha256":
		hash = sha256.New()
	default:
		return ""
	}

	file, err := os.Open(fileToHash)
	ExitIfError(err)
	defer file.Close()

	_, err = io.Copy(hash, file)
	ExitIfError(err)

	return hex.EncodeToString(hash.Sum(nil))
}

func Debug(msg string) {
	if viper.GetBool("debug") {
		log.Println("DEBUG: " + msg)
	}
}
