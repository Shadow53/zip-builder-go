package build

import (
	"encoding/xml"
	"log"
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
func makePermsFile(root string, zip *lib.ZipInfo, apps lib.Apps, files *lib.Files) {
	var exceptions Permissions
	permissionFile := make(map[string]lib.AndroidVersionInfo)
	fileInfo := lib.FileInfo{
		Destination: "/system/etc/default-permissions/" + zip.Name + "-permissions.xml",
		Mode:        "0644",
		FileName:    "permissions.xml"}

	// Generate path to permissions file
	fileDest := filepath.Join(root, "files")
	os.MkdirAll(fileDest, os.ModeDir|0755)
	fileDest = filepath.Join(fileDest, "permissions.xml")

	for _, app := range zip.Apps {
		if apps[app].PackageName != "" {
			perms := PermissionApp{Name: apps[app].PackageName}
			for _, perm := range apps[app].Permissions {
				if strings.Index(perm, ".") < 0 {
					perm = "android.permission." + perm
				}
				perms.Permissions = append(perms.Permissions, Permission{Name: perm})
			}
			exceptions.Apps = append(exceptions.Apps, perms)
		}
	}

	var minVersion string
	for _, ver := range lib.Versions {
		for _, app := range zip.Apps {
			if minVersion == "" && apps[app].AndroidVersion[ver].Base == ver {
				minVersion = ver
			}
			if minVersion != "" {
				permissionFile[ver] = lib.AndroidVersionInfo{
					Arch: make(map[string]lib.FileInfo),
					Base: minVersion}
				// Only need to set this because is not arch-specific, will be reached first
				permissionFile[ver].Arch[lib.Arches[0]] = fileInfo
			}
		}
	}

	if len(exceptions.Apps) > 0 {
		log.Println("Generating permissions file for " + zip.Name)

		file, err := os.Create(fileDest)
		lib.ExitIfError(err)
		defer file.Close()

		file.Write([]byte(xml.Header))

		enc := xml.NewEncoder(file)
		enc.Indent("  ", "    ")
		err = enc.Encode(exceptions)
		lib.ExitIfError(err)

		// File was created, add to files list for install/addon.d backup
		(*files)["permissions.xml"] = permissionFile

		zip.Files = append(zip.Files, "permissions.xml")
	}
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
 *   usage, do not restrict for this app
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

func makeSysconfigFile(root string, zip *lib.ZipInfo, apps lib.Apps, files *lib.Files) {
	var sysconfig SysConfig
	sysconfigFile := make(map[string]lib.AndroidVersionInfo)
	fileInfo := lib.FileInfo{
		Destination: "/system/etc/sysconfig/" + zip.Name + ".xml",
		Mode:        "0644",
		FileName:    "sysconfig.xml"}

	for _, app := range zip.Apps {
		if apps[app].PackageName != "" {
			a := apps[app]
			if a.DozeWhitelist {
				sysconfig.DozeWhitelist = append(sysconfig.DozeWhitelist, DozeWhitelist{Package: a.PackageName})
			}
			if a.DozeWhitelistExceptIdle {
				sysconfig.DozeWhitelistExceptIdle = append(sysconfig.DozeWhitelistExceptIdle, DozeWhitelistExceptIdle{Package: a.PackageName})
			}
			if a.DataSaverWhitelist {
				sysconfig.DataSaverWhitelist = append(sysconfig.DataSaverWhitelist, DataSaverWhitelist{Package: a.PackageName})
			}
			if a.AllowSystemUser {
				sysconfig.SystemWhitelist = append(sysconfig.SystemWhitelist, SystemWhitelistUser{Package: a.PackageName})
			}
			if a.BlacklistSystemUser {
				sysconfig.SystemBlacklist = append(sysconfig.SystemBlacklist, SystemBlacklistUser{Package: a.PackageName})
			}
		}
	}

	var minVersion string
	for _, ver := range lib.Versions {
		for _, app := range zip.Apps {
			if minVersion == "" && apps[app].AndroidVersion[ver].Base == ver {
				minVersion = ver
			}
			if minVersion != "" {
				sysconfigFile[ver] = lib.AndroidVersionInfo{
					Arch: make(map[string]lib.FileInfo),
					Base: minVersion}
				// Only need to set this because is not arch-specific, will be reached first
				sysconfigFile[ver].Arch[lib.Arches[0]] = fileInfo
			}
		}
	}

	if len(sysconfig.DozeWhitelist) > 0 || len(sysconfig.DozeWhitelistExceptIdle) > 0 || len(sysconfig.DataSaverWhitelist) > 0 ||
		len(sysconfig.SystemWhitelist) > 0 || len(sysconfig.SystemBlacklist) > 0 {

		log.Println("Generating sysconfig file")
		fileDest := filepath.Join(root, "files")
		os.MkdirAll(fileDest, os.ModeDir|0755)
		fileDest = filepath.Join(fileDest, "sysconfig.xml")

		file, err := os.Create(fileDest)
		lib.ExitIfError(err)
		defer file.Close()

		file.Write([]byte(xml.Header))

		enc := xml.NewEncoder(file)
		enc.Indent("  ", "    ")
		err = enc.Encode(sysconfig)
		lib.ExitIfError(err)

		// File was created, add to files list for install/addon.d backup
		(*files)["sysconfig.xml"] = sysconfigFile

		zip.Files = append(zip.Files, "sysconfig.xml")
	}
}
