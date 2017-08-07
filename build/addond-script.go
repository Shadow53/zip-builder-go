package build

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/Shadow53/zip-builder/lib"
)

func genAddondScript(dest string, zip lib.ZipInfo, backupFiles []string, deleteFiles map[string]bool) {
	var script bytes.Buffer
	script.WriteString(`#!/sbin/sh
#
# This addon.d script was automatically generated
# It backs up the files installed by `)

	script.WriteString(zip.Name)
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

	for file, _ := range deleteFiles {
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

	ioutil.WriteFile(dest, []byte(script.String()), 0644)
}

func makeAddondScripts(root string, zip *lib.ZipInfo, apps lib.Apps, files *lib.Files) {
	log.Println("Generating addon.d recovery script(s)")
	addondFile := make(map[string]lib.AndroidVersionInfo)
	// Set to lowest version so it is set if nothing is installed, for when a zip only removes
	baseVersion := lib.Versions[0]
VersionL:
	for _, ver := range lib.Versions {
		lib.Debug("VERSION: " + ver)
		for _, arch := range lib.Arches {
			lib.Debug("ARCH: " + arch)
			var backupFiles []string
			deleteFiles := make(map[string]bool)
			var isArchSpecific bool
			for _, app := range zip.Apps {
				if strings.HasPrefix(apps[app].AndroidVersion[ver].Arch[arch].Destination, "/system/") {
					lib.Debug("BACKING UP APP: " + apps[app].AndroidVersion[ver].Arch[arch].Destination)
					backupFiles = append(backupFiles, apps[app].AndroidVersion[ver].Arch[arch].Destination[8:])
					isArchSpecific = isArchSpecific || apps[app].AndroidVersion[ver].HasArchSpecificInfo
					if baseVersion < apps[app].AndroidVersion[ver].Base {
						baseVersion = apps[app].AndroidVersion[ver].Base
					}
				}
				for _, del := range apps[app].AndroidVersion[ver].Arch[arch].UpdateRemoveFiles {
					lib.Debug("DELETING FILE: " + del)
					deleteFiles[del] = true
				}
			}

			for _, file := range zip.Files {
				if strings.HasPrefix((*files)[file][ver].Arch[arch].Destination, "/system/") {
					lib.Debug("BACKING UP FILE: " + (*files)[file][ver].Arch[arch].Destination)
					backupFiles = append(backupFiles, (*files)[file][ver].Arch[arch].Destination[8:])
					isArchSpecific = isArchSpecific || (*files)[file][ver].HasArchSpecificInfo
					if baseVersion < (*files)[file][ver].Base {
						baseVersion = (*files)[file][ver].Base
					}
				}
				for _, del := range (*files)[file][ver].Arch[arch].UpdateRemoveFiles {
					lib.Debug("DELETING FILE: " + del)
					deleteFiles[del] = true
				}
			}

			for _, del := range zip.UpdateRemoveFiles {
				lib.Debug("DELETING FILE: " + del)
				deleteFiles[del] = true
			}

			if len(backupFiles)+len(deleteFiles) > 0 {
				scriptDest := filepath.Join(root, "files")
				os.MkdirAll(scriptDest, os.ModeDir|0755)
				fileName := "addond-" + baseVersion
				if isArchSpecific {
					fileName = fileName + "-" + arch
				}
				fileName = fileName + ".sh"
				scriptDest = filepath.Join(scriptDest, fileName)

				genAddondScript(scriptDest, *zip, backupFiles, deleteFiles)

				androidInfo := addondFile[ver]
				if androidInfo.Base == "" {
					androidInfo = lib.AndroidVersionInfo{
						Base:                baseVersion,
						HasArchSpecificInfo: isArchSpecific,
						Arch:                make(map[string]lib.FileInfo)}
				}

				androidInfo.Arch[arch] = lib.FileInfo{
					Destination: "/system/addon.d/05-" + zip.Name + ".sh",
					Mode:        "0644",
					FileName:    fileName}

				addondFile[ver] = androidInfo

				if !isArchSpecific {
					lib.Debug("NOTHING ARCH-SPECIFIC. CONTINUING")
					continue VersionL
				}
			} else {
				lib.Debug("NO FILES TO BACK UP OR DELETE. SKIPPING")
			}
		}
	}

	// File was created, add to files list for installation
	lib.Debug("ADDING ADDON.D FILE TO FILE LIST")
	(*files)["addond"] = addondFile

	lib.Debug("ADDING ADDON.D FILE TO ZIP FILE LIST")
	zip.Files = append(zip.Files, "addond")
}
