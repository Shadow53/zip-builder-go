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
    list_files | while read FILE REPLACEMENT; do
      echo "Restoring $REPLACEMENT"
      R=""
      [ -n "$REPLACEMENT" ] && R="$S/$REPLACEMENT"
      [ -f "$C/$S/$FILE" ] && restore_file $S/"$FILE" "$R"
    done
`)

	for file := range deleteFiles {
		script.WriteString("  rm -r ")
		script.WriteString(file)
		script.WriteString("\n")
	}
	script.WriteString(`  ;;
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
	baseVersion := lib.Versions[0]
	var baseMux sync.Mutex

	zip.Mux.RLock()
	zipApps := zip.Apps
	zipFiles := zip.Files
	zip.Mux.RUnlock()
	var wg sync.WaitGroup
	wg.Add(len(lib.Versions) * len(lib.Arches) * (len(zipApps) + len(zipFiles)))

	for _, ver := range lib.Versions {
		lib.Debug("VERSION: " + ver)
		for _, arch := range lib.Arches {
			lib.Debug("ARCH: " + arch)
			var backupFiles []string
			var backupMux sync.Mutex
			deleteFiles := make(map[string]bool)
			var deleteMux sync.Mutex
			var isArchSpecific bool
			var archSpecificMux sync.Mutex

			for _, app := range zipApps {
				go func(ver, arch, app string, baseVersion *string, backupFiles *[]string, deleteFiles *map[string]bool, isArchSpecific *bool, baseMux, backupMux, deleteMux, archSpecificMux *sync.Mutex, wg *sync.WaitGroup) {
					defer wg.Done()

					lib.Debug("Processing " + app + " for backup")
					apps.Mux.RLock()
					apps.App[app].Mux.RLock()
					apps.App[app].Android.Mux.RLock()
					if apps.App[app].Android.Version[ver] != nil {
						apps.App[app].Android.Version[ver].Mux.RLock()
						if apps.App[app].Android.Version[ver].HasArchSpecificInfo || arch == lib.Arches[0] {
							if apps.App[app].Android.Version[ver].Arch[arch] != nil {
								apps.App[app].Android.Version[ver].Arch[arch].Mux.RLock()

								if strings.HasPrefix(apps.App[app].Android.Version[ver].Arch[arch].Destination, "/system/") {
									lib.Debug("BACKING UP APP: " + apps.App[app].Android.Version[ver].Arch[arch].Destination)
									backupMux.Lock()
									*backupFiles = append(*backupFiles, apps.App[app].Android.Version[ver].Arch[arch].Destination[8:])
									backupMux.Unlock()
									archSpecificMux.Lock()
									*isArchSpecific = *isArchSpecific || apps.App[app].Android.Version[ver].HasArchSpecificInfo
									archSpecificMux.Unlock()
									baseMux.Lock()
									if *baseVersion < apps.App[app].Android.Version[ver].Base {
										*baseVersion = apps.App[app].Android.Version[ver].Base
									}
									baseMux.Unlock()
								}
								deleteMux.Lock()
								for _, del := range apps.App[app].Android.Version[ver].Arch[arch].UpdateRemoveFiles {
									lib.Debug("DELETING FILE: " + del)
									(*deleteFiles)[del] = true
								}
								deleteMux.Unlock()
								apps.App[app].Android.Version[ver].Arch[arch].Mux.RUnlock()
							}
						}
						apps.App[app].Android.Version[ver].Mux.RUnlock()
					}
					apps.App[app].Android.Mux.RUnlock()
					apps.App[app].Mux.RUnlock()
					apps.Mux.RUnlock()
				}(ver, arch, app, &baseVersion, &backupFiles, &deleteFiles, &isArchSpecific, &baseMux, &backupMux, &deleteMux, &archSpecificMux, &wg)
			}

			for _, file := range zipFiles {
				go func(ver, arch, file string, baseVersion *string, backupFiles *[]string, deleteFiles *map[string]bool, isArchSpecific *bool, baseMux, backupMux, deleteMux, archSpecificMux *sync.Mutex, wg *sync.WaitGroup) {
					defer wg.Done()
					lib.Debug("Processing " + file + " for backup")
					files.Mux.RLock()
					files.File[file].Mux.RLock()
					if files.File[file].Version[ver] != nil {
						files.File[file].Version[ver].Mux.RLock()
						if files.File[file].Version[ver].HasArchSpecificInfo || arch == lib.Arches[0] {
							if files.File[file].Version[ver].Arch[arch] != nil {
								files.File[file].Version[ver].Arch[arch].Mux.RLock()
								if strings.HasPrefix(files.File[file].Version[ver].Arch[arch].Destination, "/system/") {
									lib.Debug("BACKING UP FILE: " + files.File[file].Version[ver].Arch[arch].Destination)
									backupMux.Lock()
									*backupFiles = append(*backupFiles, files.File[file].Version[ver].Arch[arch].Destination[8:])
									backupMux.Unlock()
									archSpecificMux.Lock()
									*isArchSpecific = *isArchSpecific || files.File[file].Version[ver].HasArchSpecificInfo
									archSpecificMux.Unlock()
									baseMux.Lock()
									if *baseVersion < files.File[file].Version[ver].Base {
										*baseVersion = files.File[file].Version[ver].Base
									}
									baseMux.Unlock()
								}
								deleteMux.Lock()
								for _, del := range files.File[file].Version[ver].Arch[arch].UpdateRemoveFiles {
									lib.Debug("DELETING FILE: " + del)
									(*deleteFiles)[del] = true
								}
								deleteMux.Unlock()
								files.File[file].Version[ver].Arch[arch].Mux.RUnlock()
							}
						}
						files.File[file].Version[ver].Mux.RUnlock()
					}
					files.File[file].Mux.RUnlock()
					files.Mux.RUnlock()
				}(ver, arch, file, &baseVersion, &backupFiles, &deleteFiles, &isArchSpecific, &baseMux, &backupMux, &deleteMux, &archSpecificMux, &wg)
			}

			zip.Mux.RLock()
			deleteMux.Lock()
			for _, del := range zip.UpdateRemoveFiles {
				lib.Debug("DELETING FILE: " + del)
				deleteFiles[del] = true
			}
			deleteMux.Unlock()
			zip.Mux.RUnlock()

			if len(backupFiles)+len(deleteFiles) > 0 {
				scriptDest := filepath.Join(root, "files")
				err := os.MkdirAll(scriptDest, os.ModeDir|0755)
				if err != nil {
					return fmt.Errorf("Error while making parent directories for %v:\n  %v", scriptDest, err)
				}
				fileName := "addond-" + baseVersion
				if isArchSpecific {
					fileName = fileName + "-" + arch
				}
				fileName = fileName + ".sh"
				scriptDest = filepath.Join(scriptDest, fileName)

				err = genAddondScript(scriptDest, zip, backupFiles, deleteFiles)
				if err != nil {
					lib.Debug("ADDON.D GENERATION FAILED")
					return fmt.Errorf("Error while generating the addon.d survival script for %v:\n  %v", zip.Name, err)
				} else {
					lib.Debug("SUCCESSFULLY GENERATED ADDON.D AT " + scriptDest)
				}

				var androidInfo = &lib.AndroidVersionInfo{
					Base:                baseVersion,
					HasArchSpecificInfo: isArchSpecific,
					Arch:                make(map[string]*lib.FileInfo)}

				zip.Mux.RLock()
				androidInfo.Arch[arch] = &lib.FileInfo{
					Destination: "/system/addon.d/05-" + zip.Name + ".sh",
					Mode:        "0644",
					FileName:    fileName}
				zip.Mux.RUnlock()

				addondFile[ver] = androidInfo
			} else {
				lib.Debug("NO FILES TO BACK UP OR DELETE. SKIPPING")
			}
		}
	}

	lib.Debug("AWAITING APP AND FILE BACKUP PROCESSING")
	wg.Wait()
	lib.Debug("APP AND FILE BACKUP PROCESSING COMPLETE")
	// File was created, add to files list for installation
	lib.Debug("ADDING ADDON.D FILE TO FILE LIST")
	files.Mux.Lock()
	files.File["addond"] = &lib.AndroidVersions{}
	files.File["addond"].Mux.Lock()
	files.File["addond"].Version = addondFile
	files.File["addond"].Mux.Unlock()
	files.Mux.Unlock()

	lib.Debug("ADDING ADDON.D FILE TO ZIP FILE LIST")
	zip.Mux.Lock()
	zip.Files = append(zip.Files, "addond")
	zip.Mux.Unlock()
	return nil
}
