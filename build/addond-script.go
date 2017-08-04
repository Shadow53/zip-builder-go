package build

import (
    "bytes"
    "io/ioutil"
    "lib"
    "log"
    "os"
    "path/filepath"
    "strings"
)

func makeAddondScript(root string, zip *lib.ZipInfo, apps map[string]lib.AppInfo, files *map[string]lib.FileInfo) {
    log.Println("Generating addon.d recovery script")
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
        if strings.HasPrefix((*files)[file].Destination, "/system/") {
            backupFiles = append(backupFiles, (*files)[file].Destination[8:])
            for _, del := range (*files)[file].UpdateRemoveFiles {
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
    
    // File was created, add to files list for installation
    (*files)["addond"] = lib.FileInfo{
        Destination: "/system/addon.d/05-" + zip.Name + ".sh",
        Mode:        "0644",
        FileName:    "addond.sh" }
    
    zip.Files = append(zip.Files, "addond")    
} 
