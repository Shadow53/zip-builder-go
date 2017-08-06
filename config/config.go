package config

import (
    "lib"
    "log"
    "strings"
    "github.com/spf13/viper"
)

func parseFileConfig(file map[string]interface{}) lib.FileInfo {
    dest := lib.StringOrDefault(file["destination"], "")
    start := strings.LastIndex(dest, "/") + 1
    name := lib.StringOrDefault(file["package_name"], "") + ".apk"
    if name == ".apk" && start > -1 {
        name = dest[start:]
    }
    return lib.FileInfo{
        Url:                lib.StringOrDefault(file["url"], ""),
        Destination:        dest,
        InstallRemoveFiles: lib.StringSliceOrNil(file["install_remove_files"]),
        UpdateRemoveFiles:  lib.StringSliceOrNil(file["update_remove_files"]),
        Hash:               lib.StringOrDefault(file["hash"], ""),
        Mode:               lib.StringOrDefault(file["mode"], "0644"),
        FileName:           name}
}

func mergeFileConfig(file *lib.FileInfo, toMerge lib.FileInfo) {
    if toMerge.Url                != ""  { file.Url                = toMerge.Url }
    if toMerge.Destination        != ""  { file.Destination        = toMerge.Destination }
    if toMerge.InstallRemoveFiles != nil { file.InstallRemoveFiles = toMerge.InstallRemoveFiles }
    if toMerge.UpdateRemoveFiles  != nil { file.UpdateRemoveFiles  = toMerge.UpdateRemoveFiles }
    if toMerge.Hash               != ""  { file.Hash               = toMerge.Hash }
    if toMerge.Mode               != ""  { file.Mode               = toMerge.Mode }
    if toMerge.FileName           != ""  { file.FileName           = toMerge.FileName }
}

func parseAndroidVersionConfig(app map[string]interface{}) map[string]lib.AndroidVersionInfo {
    versionInfo := make(map[string]lib.AndroidVersionInfo)
    var versionSet bool
    var versionsArr, hasVersions = app["androidversion"].([]interface{})
    if !hasVersions { log.Fatal(app["name"].(string) + " must have at least one \"androidversion\" configured") }
    appConfig := parseFileConfig(app)
    for i, ver := range lib.Versions {
        for _, verInterface := range versionsArr {
            version, ok := verInterface.(map[string]interface{})
            if ok && lib.StringOrDefault(version["number"], "") == ver {
                versionSet = true
                info := lib.AndroidVersionInfo{ Base: ver, Arch: make(map[string]lib.FileInfo) }
                archInfoArr, ok2 := version["arch"].([]interface{})
                for _, arch := range lib.Arches {
                    info.HasArchSpecificInfo = info.HasArchSpecificInfo || version["arch"] != nil
                    
                    // Outer app config
                    fConfig := appConfig
                    // Android version-specific config
                    mergeFileConfig(&fConfig, parseFileConfig(version))
                    // Arch-specific config
                    if ok2 {
                        for _, aInfo := range archInfoArr {
                            archInfo := aInfo.(map[string]interface{})
                            if archInfo["arch"].(string) == arch {
                                mergeFileConfig(&fConfig, parseFileConfig(archInfo))
                            }
                        }
                    } else {
                        if version["arch"] != nil {
                            lib.Debug("COULD NOT PARSE ARCH INFORMATION FOR " + lib.StringOrDefault(app["name"], "NOTFOUND"))
                        }
                    }
                    
                    info.Arch[arch] = fConfig
                }
                
                // Set values for this and later Android versions
                for _, ver2 := range lib.Versions[i:] {
                    versionInfo[ver2] = info
                }
            } else {
                if !ok {
                    lib.Debug("COULD NOT CONVERT \"androidversion\" PROPERLY: " + lib.StringOrDefault(app["name"], "NOTFOUND"))
                }
            }
        }
    }
    if !versionSet {
        log.Fatalln("No minimum Android version found! Please set a minimum Android version for app \"" + lib.StringOrDefault(app["name"], "NOT FOUND") + "\"")
    }
    return versionInfo
}

func parseAppConfig(app map[string]interface{}) lib.AppInfo {
    return lib.AppInfo{
        PackageName:             lib.StringOrDefault(app["package_name"], ""),
        UrlIsFDroidRepo:         lib.BoolOrDefault(app["is_fdroid_repo"], false),
        DozeWhitelist:           lib.BoolOrDefault(app["doze_whitelist"], false),
        DozeWhitelistExceptIdle: lib.BoolOrDefault(app["doze_whitelist_except_idle"], false),
        DataSaverWhitelist:      lib.BoolOrDefault(app["data_saver_whitelist"], false),
        AllowSystemUser:         lib.BoolOrDefault(app["grant_system_user"], false),
        BlacklistSystemUser:     lib.BoolOrDefault(app["blacklist_system_user"], false),
        AndroidVersion:          parseAndroidVersionConfig(app),
        Permissions:             lib.StringSliceOrNil(app["permissions"])}
}

func parseZipConfig(zip map[string]interface{}) lib.ZipInfo {
    return lib.ZipInfo{
        Name:               lib.StringOrDefault(zip["name"], ""),
        Arch:               lib.StringOrDefault(zip["arch"], ""),
        SdkVersion:         lib.StringOrDefault(zip["android_sdk"], ""),
        InstallRemoveFiles: lib.StringSliceOrNil(zip["install_remove_files"]),
        UpdateRemoveFiles:  lib.StringSliceOrNil(zip["update_remove_files"]),
        Apps:               lib.StringSliceOrNil(zip["apps"]),
        Files:              lib.StringSliceOrNil(zip["files"])}
}

// TODO: Throw exceptions if values are not as expected
func MakeConfig() ([]lib.ZipInfo, lib.Apps, lib.Files) {
    // Read data from config into memory
    log.Print("Loading configuration...")
    configApps := viper.Get("apps").([]interface{})
    apps := make(lib.Apps)
    for _, a := range configApps {
        app := a.(map[string]interface{})
        if lib.StringOrDefault(app["name"], "") == "" {
            lib.Debug("WARNING: APP " + lib.StringOrDefault(app["name"], "NONAME") + " MISSING \"name\"")
        } else if lib.StringOrDefault(app["package_name"], "") == "" {
            lib.Debug("WARNING: APP " + lib.StringOrDefault(app["name"], "NONAME") + " MISSING \"package_name\"")
        } else {
            apps[app["name"].(string)] = parseAppConfig(app)
        }
    }
    
    configFiles := viper.Get("files").([]interface{})
    files := make(lib.Files)
    for _, f := range configFiles {
        file := f.(map[string]interface{})
        if lib.StringOrDefault(file["name"], "") == "" {
            lib.Debug("WARNING: FILE \"" + lib.StringOrDefault(file["name"], "NONAME") + "\" MISSING \"name\"")         
        } else {
            files[file["name"].(string)] = parseAndroidVersionConfig(file)
        }
    }
    
    configZips := viper.Get("zips").([]interface{})
    var zips []lib.ZipInfo
    for _, z := range configZips {
        zip := z.(map[string]interface{})
        zips = append(zips, parseZipConfig(zip))
    }
    log.Println("Loaded")
    return zips, apps, files
} 
