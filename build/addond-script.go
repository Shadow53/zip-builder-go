package build

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gitlab.com/Shadow53/zip-builder/lib"
)

func genAddondScript(dest string, zip *lib.ZipInfo, backupFiles []string, deleteFiles map[string]bool) error {
	lib.Debug("GENERATING ADDON.D AT " + dest)
	var script bytes.Buffer
	script.WriteString(`#!/sbin/sh
#
# This addon.d script was automatically generated
# It backs up the files installed by `)

	zip.Mux.RLock()
	script.WriteString(zip.Name)
	zip.Mux.RUnlock()
	script.WriteString(`.zip
# If there are any issues, send an email to admin@shadow53.com
# describing the issue
#

. /tmp/backuptool.functions

list_files() {
  cat <<EOF
`)

	script.WriteString(strings.Join(backupFiles, "\n"))

	script.WriteString(`
EOF
}

case "$1" in
  backup)
    list_files | while read FILE DUMMY; do
      echo "Backing up $FILE"
      backup_file $S/"$FILE"
    done
  ;;
  restore)
`)

	for file := range deleteFiles {
		script.WriteString("  rm -r ")
		script.WriteString(file)
		script.WriteString("\n")
	}
	script.WriteString(`  list_files | while read FILE REPLACEMENT; do
    echo "Restoring $REPLACEMENT"
    R=""
    [ -n "$REPLACEMENT" ] && R="$S/$REPLACEMENT"
    [ -f "$C/$S/$FILE" ] && restore_file $S/"$FILE" "$R"
  done
  ;;
  pre-backup)
  #Stub
  ;;
  post-backup)
  #Stub
  ;;
  pre-restore)
  #Stub
  ;;
  post-restore)
  #Stub
  ;;
esac`)

	err := ioutil.WriteFile(dest, []byte(script.String()), 0644)
	if err != nil {
		return fmt.Errorf("Error while writing addon.d survival script to %v:\n  %v", dest, err)
	}
	return nil
}

func makeAddondScripts(root string, zip *lib.ZipInfo, apps *lib.Apps, files *lib.Files) error {
	fmt.Println("Generating addon.d recovery script(s)")
	addondFile := make(map[string]*lib.AndroidVersionInfo)
	// Set to lowest version so it is set if nothing is installed, for when a zip only removes
	baseVersion := zip.Versions[0]
	var baseMux sync.Mutex

	zip.RLock()
	zipApps := zip.Apps
	zipFiles := zip.Files
	zip.RUnlock()
	var wg sync.WaitGroup

	for _, ver := range zip.Versions {
		lib.Debug("VERSION: " + ver)
		for _, arch := range zip.Arches {
			lib.Debug("ARCH: " + arch)
			var backupFiles []string
			var backupMux sync.Mutex
			deleteFiles := make(map[string]bool)
			var deleteMux sync.Mutex
			var isArchSpecific bool
			var archSpecificMux sync.Mutex

			wg.Add(1)
			var archwg sync.WaitGroup

			for _, app := range zipApps {
				archwg.Add(1)
				go func(ver, arch, app string, baseVersion *string, backupFiles *[]string, deleteFiles *map[string]bool, isArchSpecific *bool, baseMux, backupMux, deleteMux, archSpecificMux *sync.Mutex, wg *sync.WaitGroup) {
					defer wg.Done()
					lib.Debug("Processing " + app + " for backup")
					apps.RLockApp(app)
					defer apps.RUnlockApp(app)
					if apps.AppVersionExists(app, ver) {
						apps.RLockAppVersion(app, ver)
						defer apps.RUnlockAppVersion(app, ver)
						if !apps.GetAppVersion(app, ver).HasArchSpecificInfo {
							arch = lib.NOARCH
						}
						if apps.AppVersionArchExists(app, ver, arch) {
							apps.RLockAppVersionArch(app, ver, arch)
							defer apps.RUnlockAppVersionArch(app, ver, arch)
							if strings.HasPrefix(apps.GetAppVersionArch(app, ver, arch).Destination, "/system/") {
								lib.Debug("BACKING UP APP: " + apps.GetAppVersionArch(app, ver, arch).Destination)
								backupMux.Lock()
								*backupFiles = append(*backupFiles, apps.GetAppVersionArch(app, ver, arch).Destination[8:])
								backupMux.Unlock()
								archSpecificMux.Lock()
								*isArchSpecific = *isArchSpecific || apps.GetAppVersion(app, ver).HasArchSpecificInfo
								archSpecificMux.Unlock()
								baseMux.Lock()
								if *baseVersion < apps.GetAppVersion(app, ver).Base {
									*baseVersion = apps.GetAppVersion(app, ver).Base
								}
								baseMux.Unlock()
							} else {
								lib.Debug(app + " IS NOT IN /system/")
							}
							deleteMux.Lock()
							for _, del := range apps.GetAppVersionArch(app, ver, arch).UpdateRemoveFiles {
								lib.Debug("DELETING FILE: " + del)
								(*deleteFiles)[del] = true
							}
							deleteMux.Unlock()
						}
					}
				}(ver, arch, app, &baseVersion, &backupFiles, &deleteFiles, &isArchSpecific, &baseMux, &backupMux, &deleteMux, &archSpecificMux, &archwg)
			}

			for _, file := range zipFiles {
				archwg.Add(1)
				go func(ver, arch, file string, baseVersion *string, backupFiles *[]string, deleteFiles *map[string]bool, isArchSpecific *bool, baseMux, backupMux, deleteMux, archSpecificMux *sync.Mutex, wg *sync.WaitGroup) {
					defer wg.Done()
					lib.Debug("Processing " + file + " for backup")
					files.RLockFile(file)
					defer files.RUnlockFile(file)
					if files.FileVersionExists(file, ver) {
						files.RLockFileVersion(file, ver)
						defer files.RUnlockFileVersion(file, ver)
						if !files.GetFileVersion(file, ver).HasArchSpecificInfo {
							arch = lib.NOARCH
						}
						if files.FileVersionArchExists(file, ver, arch) {
							files.RLockFileVersionArch(file, ver, arch)
							defer files.RUnlockFileVersionArch(file, ver, arch)
							if strings.HasPrefix(files.GetFileVersionArch(file, ver, arch).Destination, "/system/") {
								lib.Debug("BACKING UP FILE: " + files.GetFileVersionArch(file, ver, arch).Destination)
								backupMux.Lock()
								*backupFiles = append(*backupFiles, files.GetFileVersionArch(file, ver, arch).Destination[8:])
								backupMux.Unlock()
								archSpecificMux.Lock()
								*isArchSpecific = *isArchSpecific || files.GetFileVersion(file, ver).HasArchSpecificInfo
								archSpecificMux.Unlock()
								baseMux.Lock()
								if *baseVersion < files.GetFileVersion(file, ver).Base {
									*baseVersion = files.GetFileVersion(file, ver).Base
								}
								baseMux.Unlock()
							}
							deleteMux.Lock()
							for _, del := range files.GetFileVersionArch(file, ver, arch).UpdateRemoveFiles {
								lib.Debug("DELETING FILE: " + del)
								(*deleteFiles)[del] = true
							}
							deleteMux.Unlock()
						}
					}
				}(ver, arch, file, &baseVersion, &backupFiles, &deleteFiles, &isArchSpecific, &baseMux, &backupMux, &deleteMux, &archSpecificMux, &archwg)
			}

			zip.RLock()
			deleteMux.Lock()
			for _, del := range zip.UpdateRemoveFiles {
				lib.Debug("DELETING FILE: " + del)
				deleteFiles[del] = true
			}
			deleteMux.Unlock()
			zip.RUnlock()

			archwg.Wait()

			if len(backupFiles)+len(deleteFiles) > 0 {
				scriptDest := filepath.Join(root, "files")
				err := os.MkdirAll(scriptDest, os.ModeDir|0755)
				if err != nil {
					return fmt.Errorf("Error while making parent directories for %v:\n  %v", scriptDest, err)
				}
				fileName := "addond-" + baseVersion
				if isArchSpecific {
					fileName = fileName + "-" + arch
				} else {
					arch = lib.NOARCH
				}
				fileName = fileName + ".sh"
				scriptDest = filepath.Join(scriptDest, fileName)

				err = genAddondScript(scriptDest, zip, backupFiles, deleteFiles)
				if err != nil {
					lib.Debug("ADDON.D GENERATION FAILED")
					return fmt.Errorf("Error while generating the addon.d survival script for %v:\n  %v", zip.Name, err)
				}
				lib.Debug("SUCCESSFULLY GENERATED ADDON.D AT " + scriptDest)

				if addondFile[ver] == nil {
					addondFile[ver] = &lib.AndroidVersionInfo{
						Base:                baseVersion,
						HasArchSpecificInfo: isArchSpecific,
						Arch:                make(map[string]*lib.FileInfo)}
				}
				addondFile[ver].HasArchSpecificInfo = addondFile[ver].HasArchSpecificInfo || isArchSpecific
				zip.RLock()
				addondFile[ver].Arch[arch] = &lib.FileInfo{
					Destination: "/system/addon.d/05-" + zip.Name + ".sh",
					Mode:        "0644",
					FileName:    fileName}
				zip.RUnlock()
			} else {
				lib.Debug("NO FILES TO BACK UP OR DELETE. SKIPPING")
			}
			wg.Done()
		}
	}

	lib.Debug("AWAITING APP AND FILE BACKUP PROCESSING")
	wg.Wait()
	lib.Debug("APP AND FILE BACKUP PROCESSING COMPLETE")
	// File was created, add to files list for installation
	lib.Debug("ADDING ADDON.D FILE TO FILE LIST")
	zip.RLock()
	fileId := zip.Name + "-addond"
	zip.RUnlock()
	files.Lock()
	files.SetFile(fileId, &lib.AndroidVersions{})
	files.Unlock()

	files.LockFile(fileId)
	files.GetFile(fileId).Version = addondFile
	files.UnlockFile(fileId)

	lib.Debug("ADDING ADDON.D FILE TO ZIP FILE LIST")
	zip.Lock()
	zip.Files = append(zip.Files, fileId)
	zip.Unlock()
	return nil
}
