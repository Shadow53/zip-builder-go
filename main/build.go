package main

import (
    "bytes"
	"dl"
    "io/ioutil"
	"lib"
    "log"
	"os"
	"path/filepath"
    "strings"
	"github.com/spf13/viper"
)

// TODO: Add support for arch-specific and Android version-specific files
func makeFileInstallScriptlet(file lib.FileInfo, buffer *bytes.Buffer) {
    // Create the parent directories of the file and set their metadata
    destParent := file.Destination[0:strings.LastIndex(file.Destination, "/")]
    buffer.WriteString("assert(run_program(\"/system/xbin/busybox\", \"mkdir\", \"-p\", \"")
    buffer.WriteString(destParent)
    buffer.WriteString("\") == 0);\n")
    buffer.WriteString("set_metadata_recursive(\"")
    buffer.WriteString(destParent)
    buffer.WriteString("\", \"uid\", 0, \"gid\", 0, \"fmode\", 0644, \"dmode\", 0755) == \"\");\n")
    // Extract the file and assert it was extracted successfully
    buffer.WriteString("assert(package_extract_file(\"files/")
    buffer.WriteString(file.FileName)
    buffer.WriteString("\", \"")
    buffer.WriteString(file.Destination)
    buffer.WriteString("\") == \"t\");\n")
    // Set metadata for the file and assert that was successful
    buffer.WriteString("assert(set_metadata(\"")
    buffer.WriteString(file.Destination)
    buffer.WriteString("\", \"uid\", 0, \"gid\", 0, \"mode\", ")
    buffer.WriteString(file.Mode)
    buffer.WriteString(") == \"\");\n")
}

func makeUpdaterScript(root string, zip lib.ZipInfo, apps map[string]lib.AppInfo, files map[string]lib.FileInfo) {
    // This variable holds a "set" of files to be deleted
    filesToDelete := make(map[string]bool)

    var extractFiles bytes.Buffer
    var hasAddond bool
    
    // Add this zip's apps to the extraction list
    for _, app := range zip.Apps {
        if apps[app].PackageName != "" {
            makeFileInstallScriptlet(apps[app].FileInfo, &extractFiles)
            // Add any files that this app wants deleted
            hasAddond = hasAddond || len(apps[app].FileInfo.UpdateRemoveFiles) > 0
            for _, del := range apps[app].FileInfo.InstallRemoveFiles {
                filesToDelete[del] = true
            }
        }
    }
    
    // Add the other files to the extraction list
    for _, file := range zip.Files {
        if files[file].Url != "" {
            makeFileInstallScriptlet(files[file], &extractFiles)
            // Add any files that this app wants deleted
            hasAddond = hasAddond || len(files[file].UpdateRemoveFiles) > 0
            for _, del := range files[file].InstallRemoveFiles {
                filesToDelete[del] = true
            }
        }
    }
    
    hasAddond = hasAddond || len(zip.UpdateRemoveFiles) > 0
    for _, del := range zip.InstallRemoveFiles {
        filesToDelete[del] = true
    }
    
    if hasAddond {
        makeFileInstallScriptlet(
            lib.FileInfo{
                Destination: "/system/addon.d/01-" + zip.Name,
                Mode:        "0644",
                FileName:    "addond.sh"},
            &extractFiles)
    }
    
    // TODO: Dynamically decide which partitions need to be mounted
    var script bytes.Buffer
    script.WriteString(`ui_print("--------------------------------------");
ui_print("Mounting system");
ifelse(is_mounted("/system"), unmount("/system"));
run_program("/sbin/busybox", "mount", "/system");
`)
    
    for file, _ := range filesToDelete {
        // The weird spacing should cause a nice tree structure in the output
        // The generated code should recursively delete directories and normal delete files
        // TODO: Add output telling what is happening
        script.WriteString("if run_program(\"/system/xbin/busybox\", \"[\", \"-d\", \"")
        script.WriteString(file)
        script.WriteString("\", \"]\") then\n    delete_recursive(\"")
        script.WriteString(file)
        script.WriteString("\")\nelse\n    if run_program(\"/system/xbin/busybox\", \"[\", \"-f\", \"")
        script.WriteString("\", \"]\") then\n        delete(\"")
        script.WriteString(file)
        script.WriteString("\")\n    endif\nendif\n")
    }
    
    log.Println("Buffer contents:")
    log.Println(extractFiles.String())
    script.WriteString(extractFiles.String())

    script.WriteString(`ui_print("Unmounting /system");
unmount("/system");
ui_print("Done!");
ui_print("--------------------------------------");
`)
    
    scriptDest := filepath.Join(root, "/META-INF/com/google/android")
    os.MkdirAll(scriptDest, os.ModeDir | 0755)
    scriptDest = filepath.Join(scriptDest, "updater-script")
    
    ioutil.WriteFile(scriptDest, []byte(script.String()), 0644)
}

func makeAddondScript(root string, zip lib.ZipInfo, apps map[string]lib.AppInfo, files map[string]lib.FileInfo) {
    var backupFiles []string
    var deleteFiles map[string]bool
    
    for _, app := range zip.Apps {
        if strings.HasPrefix(apps[app].FileInfo.Destination, "/system/") {
            backupFiles = append(backupFiles, apps[app].FileInfo.Destination[8:])
            for _, del := range apps[app].FileInfo.UpdateRemoveFiles {
                deleteFiles[del] = true
            }
        }
    }
    
    for _, file := range zip.Files {
        if strings.HasPrefix(files[file].Destination, "/system/") {
            backupFiles = append(backupFiles, files[file].Destination[8:])
            for _, del := range files[file].UpdateRemoveFiles {
                deleteFiles[del] = true
            }
        }
    }
    
    for _, del := range zip.UpdateRemoveFiles {
        deleteFiles[del] = true
    }
    
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

case "\$1" in
backup)
list_files | while read FILE DUMMY; do
echo "Backing up \$FILE"
backup_file \$S/"\$FILE"
done
;;
restore)
list_files | while read FILE REPLACEMENT; do
echo "Restoring \$REPLACEMENT"
R=""
[ -n "\$REPLACEMENT" ] && R="\$S/\$REPLACEMENT"
[ -f "\$C/\$S/\$FILE" -o -L "\$C/\$S/\$FILE" ] && restore_file \$S/"\$FILE" "\$R"
done`)
    
    for file, _ := range deleteFiles {
        script.WriteString("rm -r ")
        script.WriteString(file)
        script.WriteString("\n")
    }
script.WriteString(`;;
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

    scriptDest := filepath.Join(root, "files")
    os.MkdirAll(scriptDest, os.ModeDir | 0755)
    scriptDest = filepath.Join(scriptDest, "addond.sh")

    ioutil.WriteFile(scriptDest, []byte(script.String()), 0644)

}

// TODO: Change app dl-ing to error if app doesn't exist
func MakeZip(zip lib.ZipInfo, apps map[string]lib.AppInfo, files map[string]lib.FileInfo) {
	zippath := filepath.Join(viper.GetString("tempdir"), "build", zip.Name)
    // Build zip root with files subdir
    err := os.MkdirAll(filepath.Join(zippath, "files"), os.ModeDir | 0755)
	//defer os.RemoveAll(zippath)
	lib.ExitIfError(err)

	// Download apps
	for _, app := range zip.Apps {
		if apps[app].PackageName != "" {
			apppath := filepath.Join(zippath, "files", apps[app].PackageName+".apk")
			if apps[app].UrlIsFDroidRepo {
				dl.DownloadFromFDroidRepo(apps[app], apppath)
			} else {
				dl.Download(apps[app].FileInfo.Url, apppath)
			}
			// TODO: Verify hash of file, error on mismatch
		}
	}
	// Download other files
	for _, file := range zip.Files {
		if files[file].Url != "" {
			filepath := filepath.Join(zippath, "files", files[file].FileName)
			dl.Download(files[file].Url, filepath)
			// TODO: Verify hash of file, error on mismatch
		}
	}

	makeUpdaterScript(zippath, zip, apps, files)
    makeAddondScript(zippath, zip, apps, files)
}
