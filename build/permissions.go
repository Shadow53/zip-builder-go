package build

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/Shadow53/zip-builder/lib"
)

type Permission struct {
	XMLName xml.Name `xml:"permission"`
	Name    string   `xml:"name,attr"`
	Fixed   bool     `xml:"fixed,attr"`
}

type PermissionApp struct {
	XMLName     xml.Name     `xml:"exception"`
	Name        string       `xml:"package,attr"`
	Permissions []Permission `xml:"permission"`
}

type Permissions struct {
	XMLName xml.Name        `xml:"exceptions"`
	Apps    []PermissionApp `xml:"exception"`
}

// Permissions file is not Android version-specific because any permissions
// or apps not found should end up ignored
func makePermsFile(root string, zip *lib.ZipInfo, apps *lib.Apps, files *lib.Files) error {
	var exceptions Permissions
	permissionFile := make(map[string]*lib.AndroidVersionInfo)

	zip.Mux.RLock()
	fileInfo := lib.FileInfo{
		Destination: "/system/etc/default-permissions/" + zip.Name + "-permissions.xml",
		Mode:        "0644",
		FileName:    "permissions.xml"}
	zip.Mux.RUnlock()

	// Generate path to permissions file
	fileDest := filepath.Join(root, "files")
	err := os.MkdirAll(fileDest, os.ModeDir|0755)
	if err != nil {
		return fmt.Errorf("Error while creating directory %v:\n  %v", fileDest, err)
	}
	fileDest = filepath.Join(fileDest, "permissions.xml")

	zip.Mux.RLock()
	zipApps := zip.Apps
	zip.Mux.RUnlock()
	for _, app := range zipApps {
		apps.Mux.RLock()
		apps.App[app].Mux.RLock()
		if apps.App[app].PackageName != "" {
			perms := PermissionApp{Name: apps.App[app].PackageName}
			for _, perm := range apps.App[app].Permissions {
				if strings.Index(perm, ".") < 0 {
					perm = "android.permission." + perm
				}
				perms.Permissions = append(perms.Permissions, Permission{Name: perm})
			}
			exceptions.Apps = append(exceptions.Apps, perms)
		}
		apps.App[app].Mux.RUnlock()
		apps.Mux.RUnlock()
	}

	var minVersion string
	for _, ver := range lib.Versions {
		for _, app := range zipApps {
			apps.Mux.RLock()
			apps.App[app].Mux.RLock()
			apps.App[app].Android.Mux.RLock()
			if apps.App[app].Android.Version[ver] != nil {
				apps.App[app].Android.Version[ver].Mux.RLock()
				if minVersion == "" && apps.App[app].Android.Version[ver].Base == ver {
					minVersion = ver
				}
				if minVersion != "" {
					permissionFile[ver] = &lib.AndroidVersionInfo{
						Arch: make(map[string]*lib.FileInfo),
						Base: minVersion}
					// Only need to set this because is not arch-specific, will be reached first
					permissionFile[ver].Arch[lib.Arches[0]] = &fileInfo
				}
				apps.App[app].Android.Version[ver].Mux.RUnlock()
			}
			apps.App[app].Android.Mux.RUnlock()
			apps.App[app].Mux.RUnlock()
			apps.Mux.RUnlock()
		}
	}

	if len(exceptions.Apps) > 0 {
		fmt.Println("Generating permissions file for " + zip.Name)

		file, err := os.Create(fileDest)
		if err != nil {
			return fmt.Errorf("Error while creating file %v:\n  %v", fileDest, err)
		}
		defer file.Close()

		_, err = file.Write([]byte(xml.Header))
		if err != nil {
			return fmt.Errorf("Error while writing XML header to file %v:\n  %v", fileDest, err)
		}

		enc := xml.NewEncoder(file)
		enc.Indent("", "    ")
		err = enc.Encode(exceptions)
		if err != nil {
			return fmt.Errorf("Error while writing permissions XML to %v:\n  %v", fileDest, err)
		}

		// File was created, add to files list for install/addon.d backup
		files.Mux.Lock()
		files.File["permissions.xml"] = &lib.AndroidVersions{}
		files.File["permissions.xml"].Version = permissionFile
		files.Mux.Unlock()

		zip.Files = append(zip.Files, "permissions.xml")
	}
	return nil
}

/*
 * "system-user-whitelisted-app"
 * - Grants system user privileges to the app
 * "system-user-blacklisted-app"
 * - Blacklists app from being granted system user
 *   privileges if it would normally have them
 * "allow-in-power-save-except-idle"
 * - Doze whitelisting but not App Standby
 * "allow-in-power-save"
 * - Doze *and* App Standby whitelisting, should be the same
 *   as if it was whitelisted in Android Settings
 * "allow-in-data-usage-save"
 * - If the system is restricting background data
 *   usage, do not restrict for this app. Only for priv-app.
 *
 * Other tags NOT (yet) included:
 * "group"
 * "permission"
 * "assign-permission"
 * "library"
 * "feature"
 * "unavailable-feature"
 * "app-link"
 * - Handle URLs to the app's website by default
 * "default-enabled-vr-app"
 * "backup-transport-whitelisted-service"
 * - Indicates a service that is whitelisted as a
 *   backup data transport. Not sure what that
 *   exactly means or where it is necessary
 * "disabled-until-used-preinstalled-carrier-associated-app"
 * - Do not allow the app to run until the SIM/
 *   carrier has been set up
 */

type SystemWhitelistUser struct {
	XMLName xml.Name `xml:"system-user-whitelisted-app"`
	Package string   `xml:"package,attr"`
}

type SystemBlacklistUser struct {
	XMLName xml.Name `xml:"system-user-blacklisted-app"`
	Package string   `xml:"package,attr"`
}

type DozeWhitelistExceptIdle struct {
	XMLName xml.Name `xml:"allow-in-power-save-except-idle"`
	Package string   `xml:"package,attr"`
}

type DozeWhitelist struct {
	XMLName xml.Name `xml:"allow-in-power-save"`
	Package string   `xml:"package,attr"`
}

type DataSaverWhitelist struct {
	XMLName xml.Name `xml:"allow-in-data-usage-save"`
	Package string   `xml:"package,attr"`
}

type SysConfig struct {
	XMLName                 xml.Name                  `xml:"config"`
	SystemWhitelist         []SystemWhitelistUser     `xml:"system-user-whitelisted-app"`
	SystemBlacklist         []SystemBlacklistUser     `xml:"system-user-blacklisted-app"`
	DozeWhitelistExceptIdle []DozeWhitelistExceptIdle `xml:"allow-in-power-save-except-idle"`
	DozeWhitelist           []DozeWhitelist           `xml:"allow-in-power-save"`
	DataSaverWhitelist      []DataSaverWhitelist      `xml:"allow-in-data-usage-save"`
}

func makeSysconfigFile(root string, zip *lib.ZipInfo, apps *lib.Apps, files *lib.Files) error {
	var sysconfig SysConfig
	sysconfigFile := make(map[string]*lib.AndroidVersionInfo)
	zip.Mux.RLock()
	fileInfo := lib.FileInfo{
		Destination: "/system/etc/sysconfig/" + zip.Name + ".xml",
		Mode:        "0644",
		FileName:    "sysconfig.xml"}
	zip.Mux.RUnlock()

	zip.Mux.RLock()
	zipApps := zip.Apps
	zip.Mux.RUnlock()
	for _, app := range zipApps {
		apps.Mux.RLock()
		apps.App[app].Mux.RLock()
		if apps.App[app].PackageName != "" {
			if apps.App[app].DozeWhitelist {
				sysconfig.DozeWhitelist = append(sysconfig.DozeWhitelist, DozeWhitelist{Package: apps.App[app].PackageName})
			}
			if apps.App[app].DozeWhitelistExceptIdle {
				sysconfig.DozeWhitelistExceptIdle = append(sysconfig.DozeWhitelistExceptIdle, DozeWhitelistExceptIdle{Package: apps.App[app].PackageName})
			}
			if apps.App[app].DataSaverWhitelist {
				sysconfig.DataSaverWhitelist = append(sysconfig.DataSaverWhitelist, DataSaverWhitelist{Package: apps.App[app].PackageName})
			}
			if apps.App[app].AllowSystemUser {
				sysconfig.SystemWhitelist = append(sysconfig.SystemWhitelist, SystemWhitelistUser{Package: apps.App[app].PackageName})
			}
			if apps.App[app].BlacklistSystemUser {
				sysconfig.SystemBlacklist = append(sysconfig.SystemBlacklist, SystemBlacklistUser{Package: apps.App[app].PackageName})
			}
		}
		apps.App[app].Mux.RUnlock()
		apps.Mux.RUnlock()
	}

	var minVersion string
	for _, ver := range lib.Versions {
		for _, app := range zipApps {
			apps.Mux.RLock()
			apps.App[app].Mux.RLock()
			apps.App[app].Android.Mux.RLock()
			if apps.App[app].Android.Version[ver] != nil {
				apps.App[app].Android.Version[ver].Mux.RLock()
				if minVersion == "" && apps.App[app].Android.Version[ver].Base == ver {
					minVersion = ver
				}
				if minVersion != "" {
					sysconfigFile[ver] = &lib.AndroidVersionInfo{
						Arch: make(map[string]*lib.FileInfo),
						Base: minVersion}
					// Only need to set this because is not arch-specific, will be reached first
					sysconfigFile[ver].Arch[lib.Arches[0]] = &fileInfo
				}
				apps.App[app].Android.Version[ver].Mux.RUnlock()
			}
			apps.App[app].Android.Mux.RUnlock()
			apps.App[app].Mux.RUnlock()
			apps.Mux.RUnlock()
		}
	}

	if len(sysconfig.DozeWhitelist) > 0 || len(sysconfig.DozeWhitelistExceptIdle) > 0 || len(sysconfig.DataSaverWhitelist) > 0 ||
		len(sysconfig.SystemWhitelist) > 0 || len(sysconfig.SystemBlacklist) > 0 {

		fmt.Println("Generating sysconfig file")
		fileDest := filepath.Join(root, "files")
		err := os.MkdirAll(fileDest, os.ModeDir|0755)
		if err != nil {
			return fmt.Errorf("Error while creating directory at %v:\n  %v", fileDest, err)
		}
		fileDest = filepath.Join(fileDest, "sysconfig.xml")

		file, err := os.Create(fileDest)
		if err != nil {
			return fmt.Errorf("Error while creating file %v:\n  %v", fileDest, err)
		}
		defer file.Close()

		_, err = file.Write([]byte(xml.Header))
		if err != nil {
			return fmt.Errorf("Error while writing XML header to %v:\n  %v", fileDest, err)
		}

		enc := xml.NewEncoder(file)
		enc.Indent("", "    ")
		err = enc.Encode(sysconfig)
		if err != nil {
			return fmt.Errorf("Error while writing sysconfig XML to %v:\n  %v", fileDest, err)
		}

		// File was created, add to files list for install/addon.d backup
		files.Mux.Lock()
		files.File["sysconfig.xml"] = &lib.AndroidVersions{}
		files.File["sysconfig.xml"].Version = sysconfigFile
		files.Mux.Unlock()

		zip.Mux.Lock()
		zip.Files = append(zip.Files, "sysconfig.xml")
		zip.Mux.Unlock()
	}
	return nil
}
