package lib

import (
    "crypto/md5"
    "encoding/hex"
    "io"
    "io/ioutil"
	"log"
    "os"
    "strings"

	"github.com/spf13/viper"
)

type FileInfo struct {
	Url                string
	Destination        string
	InstallRemoveFiles []string
	UpdateRemoveFiles  []string
	Hash               string
	Mode               string
	FileName           string
}

type AppInfo struct {
	PackageName             string
	UrlIsFDroidRepo         bool
	DozeWhitelist           bool
	DozeWhitelistExceptIdle bool
	DataSaverWhitelist      bool
	AllowSystemUser         bool
	BlacklistSystemUser     bool
	FileInfo                FileInfo
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

func ExitIfError(err error) {
	if err != nil {
		log.Fatalln(err)
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

func parseFileConfig(file map[string]interface{}) FileInfo {
    dest := StringOrDefault(file["destination"], "")
    start := strings.LastIndex(dest, "/") + 1
    name := StringOrDefault(file["package_name"], "") + ".apk"
    if name == ".apk" && start > -1 {
        name = dest[start:]
    }
	return FileInfo{
		Url:                StringOrDefault(file["url"], ""),
		Destination:        dest,
		InstallRemoveFiles: StringSliceOrNil(file["install_remove_files"]),
		UpdateRemoveFiles:  StringSliceOrNil(file["update_remove_files"]),
		Hash:               StringOrDefault(file["hash"], ""),
		Mode:               StringOrDefault(file["mode"], "0644"),
		FileName:           name}
}

func parseAppConfig(app map[string]interface{}) AppInfo {
	return AppInfo{
		PackageName:             StringOrDefault(app["package_name"], ""),
		UrlIsFDroidRepo:         BoolOrDefault(app["is_fdroid_repo"], false),
		DozeWhitelist:           BoolOrDefault(app["doze_whitelist"], false),
		DozeWhitelistExceptIdle: BoolOrDefault(app["doze_whitelist_except_idle"], false),
        DataSaverWhitelist:      BoolOrDefault(app["data_saver_whitelist"], false),
		AllowSystemUser:         BoolOrDefault(app["grant_system_user"], false),
		BlacklistSystemUser:     BoolOrDefault(app["blacklist_system_user"], false),
		FileInfo:                parseFileConfig(app),
		Permissions:             StringSliceOrNil(app["permissions"])}
}

func parseZipConfig(zip map[string]interface{}) ZipInfo {
	return ZipInfo{
		Name:               StringOrDefault(zip["name"], ""),
		Arch:               StringOrDefault(zip["arch"], ""),
		SdkVersion:         StringOrDefault(zip["android_sdk"], ""),
		InstallRemoveFiles: StringSliceOrNil(zip["install_remove_files"]),
		UpdateRemoveFiles:  StringSliceOrNil(zip["update_remove_files"]),
		Apps:               StringSliceOrNil(zip["apps"]),
		Files:              StringSliceOrNil(zip["files"])}
}

// TODO: Throw exceptions if values are not as expected
func MakeConfig() ([]ZipInfo, map[string]AppInfo, map[string]FileInfo) {
	// Read data from config into memory
    log.Print("Loading configuration...")
	configApps := viper.Get("apps").([]interface{})
	apps := make(map[string]AppInfo)
	for _, a := range configApps {
		app := a.(map[string]interface{})
		if app["name"].(string) != "" {
			apps[app["name"].(string)] = parseAppConfig(app)
		}
	}

	configFiles := viper.Get("files").([]interface{})
	files := make(map[string]FileInfo)
	for _, f := range configFiles {
		file := f.(map[string]interface{})
		if file["name"].(string) != "" && file["url"].(string) != "" && file["destination"].(string) != "" {
			files[file["name"].(string)] = parseFileConfig(file)
		}
	}

	configZips := viper.Get("zips").([]interface{})
	var zips []ZipInfo
	for _, z := range configZips {
		zip := z.(map[string]interface{})
		zips = append(zips, parseZipConfig(zip))
	}
    log.Println("Loaded")
	return zips, apps, files
}

func GenerateMD5File(path string) {
    log.Println("Generating MD5 file for " + path)
    file, err := os.Open(path)
    ExitIfError(err)
    defer file.Close()
    
    hash := md5.New()
    _, err = io.Copy(hash, file)
    ExitIfError(err)
    
    sum := hash.Sum(nil)
    text := hex.EncodeToString(sum) + "  " + path[strings.LastIndex(path, string(os.PathSeparator))+1:] + "\n"
    ioutil.WriteFile(path + ".md5", []byte(text), 0644)
}
