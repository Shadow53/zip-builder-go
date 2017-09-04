package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"gitlab.com/Shadow53/zip-builder/lib"
)

func parseFileConfig(file map[string]interface{}) *lib.FileInfo {
	dest := lib.StringOrDefault(file["destination"], "")
	start := strings.LastIndex(dest, "/") + 1
	name := lib.StringOrDefault(file["package_name"], "") + ".apk"
	if name == ".apk" && start > -1 {
		name = dest[start:]
	}
	return &lib.FileInfo{
		Url:                lib.StringOrDefault(file["url"], ""),
		Destination:        dest,
		InstallRemoveFiles: lib.StringSliceOrNil(file["install_remove_files"]),
		UpdateRemoveFiles:  lib.StringSliceOrNil(file["update_remove_files"]),
		MD5:                lib.StringOrDefault(file["md5"], ""),
		SHA1:               lib.StringOrDefault(file["sha1"], ""),
		SHA256:             lib.StringOrDefault(file["sha256"], ""),
		Mode:               lib.StringOrDefault(file["mode"], "0644"),
		FileName:           name}
}

func mergeFileConfig(file *lib.FileInfo, toMerge *lib.FileInfo) {
	if toMerge.Url != "" {
		file.Url = toMerge.Url
	}
	if toMerge.Destination != "" {
		file.Destination = toMerge.Destination
	}
	if toMerge.InstallRemoveFiles != nil {
		file.InstallRemoveFiles = toMerge.InstallRemoveFiles
	}
	if toMerge.UpdateRemoveFiles != nil {
		file.UpdateRemoveFiles = toMerge.UpdateRemoveFiles
	}
	if toMerge.MD5 != "" {
		file.MD5 = toMerge.MD5
	}
	if toMerge.SHA1 != "" {
		file.SHA1 = toMerge.SHA1
	}
	if toMerge.SHA256 != "" {
		file.SHA256 = toMerge.SHA256
	}
	if toMerge.Mode != "" {
		file.Mode = toMerge.Mode
	}
	if toMerge.FileName != "" {
		file.FileName = toMerge.FileName
	}
}

func parseAndroidVersionConfig(item map[string]interface{}) (map[string]*lib.AndroidVersionInfo, error) {
	versionInfo := make(map[string]*lib.AndroidVersionInfo)
	var versionSet bool
	var versionsArr, hasVersions = item["androidversion"].([]interface{})
	if !hasVersions {
		return nil, fmt.Errorf("%v must have at least one \"androidversion\" configured", item["name"])
	}
	appConfig := parseFileConfig(item)
	for i, ver := range lib.Versions {
		for _, verInterface := range versionsArr {
			version, versionOk := verInterface.(map[string]interface{})
			if versionOk && lib.StringOrDefault(version["number"], "") == ver {
				versionSet = true
				info := lib.AndroidVersionInfo{Base: ver, Arch: make(map[string]*lib.FileInfo)}
				archInfoArr, archArrOk := version["arch"].([]interface{})
				for _, arch := range lib.Arches {
					info.HasArchSpecificInfo = info.HasArchSpecificInfo || version["arch"] != nil

					// Outer app config
					fConfig := *appConfig
					// Android version-specific config
					mergeFileConfig(&fConfig, parseFileConfig(version))
					// Arch-specific config
					if archArrOk {
						for _, aInfo := range archInfoArr {
							archInfo, archInfoOk := aInfo.(map[string]interface{})
							if archInfoOk {
								archLabel, archLabelOk := archInfo["arch"].(string)
								if archLabelOk {
									if archLabel == arch {
										mergeFileConfig(&fConfig, parseFileConfig(archInfo))
									}
								} else {
									return nil, fmt.Errorf("Misconfigured \"arch\" on app %v, version %v: property \"arch\" is not a string", item["name"], ver)
								}
							} else {
								return nil, fmt.Errorf("Misconfigured \"arch\" on app %v, version %v: could not parse array item as architecture map", item["name"], ver)
							}

						}
					} else {
						if version["arch"] != nil {
							return nil, fmt.Errorf("Misconfigured \"arch\" on app %v, version %v: is not an array", item["name"], ver)
						}
					}

					info.Arch[arch] = &fConfig
				}

				// Set values for this and later Android versions
				for _, ver2 := range lib.Versions[i:] {
					versionInfo[ver2] = &info
				}
			} else if !versionOk {
				return nil, fmt.Errorf("Misconfigured \"androidversion\" on item %v: is not an array", item["name"])
			}
		}
	}
	if !versionSet {
		return nil, fmt.Errorf("\"androidversion\" specified but no version number found! Please set a minimum Android version for %v", item["name"])
	}
	return versionInfo, nil
}

func parseAppConfig(app map[string]interface{}) (*lib.AppInfo, error) {
	appInfo := lib.AppInfo{
		PackageName:             lib.StringOrDefault(app["package_name"], ""),
		UrlIsFDroidRepo:         lib.BoolOrDefault(app["is_fdroid_repo"], false),
		DozeWhitelist:           lib.BoolOrDefault(app["doze_whitelist"], false),
		DozeWhitelistExceptIdle: lib.BoolOrDefault(app["doze_whitelist_except_idle"], false),
		DataSaverWhitelist:      lib.BoolOrDefault(app["data_saver_whitelist"], false),
		AllowSystemUser:         lib.BoolOrDefault(app["grant_system_user"], false),
		BlacklistSystemUser:     lib.BoolOrDefault(app["blacklist_system_user"], false),
		Permissions:             lib.StringSliceOrNil(app["permissions"])}

	androidVersion, err := parseAndroidVersionConfig(app)
	if err != nil {
		return &appInfo, fmt.Errorf("Error while parsing Android version information:\n  %v", err)
	}
	appInfo.Android.Version = androidVersion
	return &appInfo, nil
}

func parseZipConfig(zip map[string]interface{}) lib.ZipInfo {
	return lib.ZipInfo{
		Name:               lib.StringOrDefault(zip["name"], ""),
		InstallRemoveFiles: lib.StringSliceOrNil(zip["install_remove_files"]),
		UpdateRemoveFiles:  lib.StringSliceOrNil(zip["update_remove_files"]),
		Apps:               lib.StringSliceOrNil(zip["apps"]),
		Files:              lib.StringSliceOrNil(zip["files"])}
}

// TODO: Throw exceptions if values are not as expected
func MakeConfig() ([]lib.ZipInfo, *lib.Apps, *lib.Files, error) {
	// Read data from config into memory
	fmt.Println("Loading configuration...")

	apps := &lib.Apps{}
	apps.App = make(map[string]*lib.AppInfo)
	if viper.Get("apps") != nil {
		configApps, appsOk := viper.Get("apps").([]interface{})
		if appsOk {
			for _, a := range configApps {
				app, appOk := a.(map[string]interface{})
				if appOk {
					appName := lib.StringOrDefault(app["name"], "")
					if appName == "" {
						return nil, &lib.Apps{}, &lib.Files{}, fmt.Errorf("App is missing \"name\" parameter%v", "")
					} else if lib.StringOrDefault(app["package_name"], "") == "" {
						return nil, &lib.Apps{}, &lib.Files{}, fmt.Errorf("App %v is missing \"package_name\" parameter", appName)
					} else {
						app, err := parseAppConfig(app)
						if err != nil {
							return nil, &lib.Apps{}, &lib.Files{}, fmt.Errorf("Error while parsing app config for %v:\n  %v", appName, err)
						}
						apps.App[appName] = app
					}
				}
			}
		} else {
			return nil, &lib.Apps{}, &lib.Files{}, fmt.Errorf("Could not parse \"apps\" as an array%v", "")
		}
	} else {
		lib.Debug("No app installation configurations found")
	}

	files := &lib.Files{}
	files.File = make(map[string]*lib.AndroidVersions)
	if viper.Get("files") != nil {
		configFiles, filesOk := viper.Get("files").([]interface{})
		if filesOk {
			for _, f := range configFiles {
				file := f.(map[string]interface{})
				name := lib.StringOrDefault(file["name"], "")
				if name != "" {
					fileConfig, err := parseAndroidVersionConfig(file)
					if err == nil {
						files.File[name] = &lib.AndroidVersions{}
						files.File[name].Version = fileConfig
					} else {
						return nil, &lib.Apps{}, &lib.Files{}, err
					}
				} else {
					return nil, &lib.Apps{}, &lib.Files{}, fmt.Errorf("File does not have a \"name\" set")
				}
			}
		} else {
			return nil, &lib.Apps{}, &lib.Files{}, fmt.Errorf("Could not parse \"files\" as an array")
		}
	} else {
		lib.Debug("No file installation configurations found")
	}

	var zips []lib.ZipInfo
	if viper.Get("zips") != nil {
		configZips, zipsOk := viper.Get("zips").([]interface{})
		if zipsOk {
			for _, z := range configZips {
				zip, zipOk := z.(map[string]interface{})
				if zipOk {
					zips = append(zips, parseZipConfig(zip))
				} else {
					return nil, &lib.Apps{}, &lib.Files{}, fmt.Errorf("Could not parse zip as configuration map")
				}
			}
		} else {
			return nil, &lib.Apps{}, &lib.Files{}, fmt.Errorf("Could not parse \"zips\" as an array")
		}
	} else {
		return nil, &lib.Apps{}, &lib.Files{}, fmt.Errorf("At least one item needs to be defined in the \"zips\" array")
	}

	fmt.Println("Loaded")
	return zips, apps, files, nil
}
