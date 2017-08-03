package build

import (
    "bytes"
    "io/ioutil"
    "lib"
    "os"
    "path/filepath"
    "strings"
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
    
    // Add this zip's apps to the extraction list
    for _, app := range zip.Apps {
        if apps[app].PackageName != "" {
            makeFileInstallScriptlet(apps[app].FileInfo, &extractFiles)
            // Add any files that this app wants deleted
            for _, del := range apps[app].FileInfo.InstallRemoveFiles {
                filesToDelete[del] = true
            }
        }
    }
    
    // Add the other files to the extraction list
    for _, file := range zip.Files {
        if files[file].FileName != "" {
            makeFileInstallScriptlet(files[file], &extractFiles)
            // Add any files that this app wants deleted
            for _, del := range files[file].InstallRemoveFiles {
                filesToDelete[del] = true
            }
        }
    }
    
    for _, del := range zip.InstallRemoveFiles {
        filesToDelete[del] = true
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
