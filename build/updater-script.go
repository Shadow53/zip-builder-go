package build

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/Shadow53/zip-builder/lib"
)

// TODO: Add support for arch-specific and Android version-specific files
func makeFileInstallScriptlet(file *lib.FileInfo, buffer *bytes.Buffer) {
	// Create the parent directories of the file and set their metadata
	file.Mux.RLock()
	destParent := file.Destination[0:strings.LastIndex(file.Destination, "/")]
	file.Mux.RUnlock()
	buffer.WriteString("assert(run_program(\"/sbin/busybox\", \"mkdir\", \"-p\", \"")
	buffer.WriteString(destParent)
	buffer.WriteString("\") == 0);\n")
	buffer.WriteString("assert(set_metadata_recursive(\"")
	buffer.WriteString(destParent)
	buffer.WriteString("\", \"uid\", 0, \"gid\", 0, \"fmode\", 0644, \"dmode\", 0755) == \"\");\n")
	// Tell the user what is happening
	buffer.WriteString("ui_print(\"Extracting ")
	file.Mux.RLock()
	buffer.WriteString(file.Destination)
	file.Mux.RUnlock()
	buffer.WriteString("\");\n")
	// Extract the file and assert it was extracted successfully
	buffer.WriteString("assert(package_extract_file(\"files/")
	file.Mux.RLock()
	buffer.WriteString(file.FileName)
	file.Mux.RUnlock()
	buffer.WriteString("\", \"")
	file.Mux.RLock()
	buffer.WriteString(file.Destination)
	file.Mux.RUnlock()
	buffer.WriteString("\") == \"t\");\n")
	// Set metadata for the file and assert that was successful
	buffer.WriteString("assert(set_metadata(\"")
	file.Mux.RLock()
	buffer.WriteString(file.Destination)
	file.Mux.RUnlock()
	buffer.WriteString("\", \"uid\", 0, \"gid\", 0, \"mode\", ")
	file.Mux.RLock()
	buffer.WriteString(file.Mode)
	file.Mux.RUnlock()
	buffer.WriteString(") == \"\");\n")
}

func makeFileDeleteScriptlet(filesToDelete map[string]bool, buffer *bytes.Buffer) {
	for file := range filesToDelete {
		// The weird spacing should cause a nice tree structure in the output
		// The generated code should recursively delete directories and normal delete files
		// TODO: Add output telling what is happening
		buffer.WriteString("if run_program(\"/sbin/busybox\", \"test\", \"-d\", \"")
		buffer.WriteString(file)
		buffer.WriteString("\") == 0 then\n    ui_print(\"Recursively deleting existing folder ")
		buffer.WriteString(file)
		buffer.WriteString("\") && delete_recursive(\"")
		buffer.WriteString(file)
		buffer.WriteString("\");\nelse\n    if run_program(\"/sbin/busybox\", \"test\", \"-f\", \"")
		buffer.WriteString(file)
		buffer.WriteString("\") == 0 then\n        ui_print(\"Deleting existing file ")
		buffer.WriteString(file)
		buffer.WriteString("\") && delete(\"")
		buffer.WriteString(file)
		buffer.WriteString("\");\n    endif;\nendif;\n")
	}
}

func makePerItemScriptlet(item map[string]*lib.AndroidVersionInfo, buff *bytes.Buffer) {
	multVersionTest := ""
	verFilesToDelete := make(map[string]bool)
	var archBuff bytes.Buffer
	var extractFiles bytes.Buffer
	var deleteFiles bytes.Buffer
	for i, ver := range lib.Versions {
		lib.Debug("ANDROID VERSION: " + ver)
		testVersion := "is_substring(\"" + ver + "\", file_getprop(\"/system/build.prop\", \"ro.build.version.release\"))"
		if item[ver] != nil && item[ver].Base != "" {
			if item[ver].Base != ver && i < len(lib.Versions)-1 {
				if multVersionTest != "" {
					multVersionTest = multVersionTest + " || "
				}
				multVersionTest = multVersionTest + testVersion
			} else if multVersionTest != "" {
				lib.Debug("TESTING FOR ANDROID VERSION: " + ver)
				buff.WriteString("if " + multVersionTest + " then\n")
				makeFileDeleteScriptlet(verFilesToDelete, &deleteFiles)
				buff.WriteString(deleteFiles.String())
				buff.WriteString(extractFiles.String())
				buff.WriteString(archBuff.String())
				buff.WriteString("endif;\n")
				// Clear vars
				multVersionTest = ""
				archBuff.Reset()
				extractFiles.Reset()
				deleteFiles.Reset()
			}
			if item[ver].Base == ver {
				if multVersionTest == "" {
					multVersionTest = testVersion
				}
				if !item[ver].HasArchSpecificInfo {
					if item[ver].Arch[lib.Arches[0]].FileName != "" {
						lib.Debug("INSTALLING: " + item[ver].Arch[lib.Arches[0]].FileName)
						makeFileInstallScriptlet(item[ver].Arch[lib.Arches[0]], &extractFiles)
						for _, del := range item[ver].Arch[lib.Arches[0]].InstallRemoveFiles {
							lib.Debug("DELETE FILE (VERSION): " + del)
							verFilesToDelete[del] = true
						}
					}
				} else {
					for _, arch := range lib.Arches {
						if item[ver].Arch[arch] != nil && item[ver].Arch[arch].FileName != "" {
							lib.Debug("TESTING FOR ANDROID ARCH: " + arch)
							archBuff.WriteString("if is_substring(\"")
							if arch == "arm" {
								archBuff.WriteString("armeabi")
							} else {
								archBuff.WriteString(arch)
							}
							archBuff.WriteString("\", file_getprop(\"/system/build.prop\", \"ro.product.cpu.abilist\") + file_getprop(\"/system/build.prop\", \"ro.product.cpu.abi\")) then\n")

							archFilesToDelete := make(map[string]bool)
							// Add any files that this app wants deleted
							for _, del := range item[ver].Arch[arch].InstallRemoveFiles {
								lib.Debug("DELETE FILE (ARCH): " + del)
								archFilesToDelete[del] = true
							}
							makeFileDeleteScriptlet(archFilesToDelete, &archBuff)

							lib.Debug("INSTALLING ITEM: " + item[ver].Arch[arch].FileName)
							makeFileInstallScriptlet(item[ver].Arch[arch], &archBuff)
							archBuff.WriteString("endif;\n")
						}
					}
				}
			}
		}
	}
}

func makeUpdaterScript(root string, zip *lib.ZipInfo, apps *lib.Apps, files *lib.Files) error {
	fmt.Println("Generating updater-script")

	var script bytes.Buffer

	// TODO: Dynamically decide which partitions need to be mounted
	script.WriteString(`ui_print("--------------------------------------");
ui_print("Mounting system");
ifelse(is_mounted("/system"), unmount("/system"));
run_program("/sbin/busybox", "mount", "/system");
ui_print("Mounting data");
ifelse(is_mounted("/data"), unmount("/data"));
run_program("/sbin/busybox", "mount", "/data");
`)

	filesToDelete := make(map[string]bool)
	for _, del := range zip.InstallRemoveFiles {
		filesToDelete[del] = true
	}
	makeFileDeleteScriptlet(filesToDelete, &script)

	for _, app := range zip.Apps {
		apps.Mux.RLock()
		apps.App[app].Mux.RLock()
		if apps.App[app].PackageName != "" {
			apps.App[app].Android.Mux.RLock()
			makePerItemScriptlet(apps.App[app].Android.Version, &script)
			apps.App[app].Android.Mux.RUnlock()
		}
		apps.App[app].Mux.RUnlock()
		apps.Mux.RUnlock()
	}

	for _, file := range zip.Files {
		files.Mux.RLock()
		files.File[file].Mux.RLock()
		makePerItemScriptlet(files.File[file].Version, &script)
		files.File[file].Mux.RUnlock()
		files.Mux.RUnlock()

		if file == "permissions.xml" {
			script.WriteString(`if run_program("/sbin/busybox", "test", "-d", "/data/data") == 0 then
		ui_print("---");
		ui_print("|- WARNING:");
		ui_print("|- It appears you have previously booted");
		ui_print("|- into this system. This zip includes a");
		ui_print("|- set of permissions to grant to installed");
		ui_print("|- apps by default, however default");
		ui_print("|- permissions are only applied on FIRST");
		ui_print("|- boot. You will need to manually grant");
		ui_print("|- permissions to these apps.");
		ui_print("---");
endif;
`)
		}
	}

	script.WriteString(`ui_print("Unmounting /system");
ui_print("Done!");
ui_print("--------------------------------------");
unmount("/system");
`)

	scriptDest := filepath.Join(root, "/META-INF/com/google/android")
	err := os.MkdirAll(scriptDest, os.ModeDir|0755)
	if err != nil {
		return fmt.Errorf("Error while creating directory %v:\n  %v", scriptDest, err)
	}
	scriptDest = filepath.Join(scriptDest, "updater-script")

	err = ioutil.WriteFile(scriptDest, []byte(script.String()), 0644)
	if err != nil {
		return fmt.Errorf("Error while writing updater-script to %v:\n  %v", scriptDest, err)
	}
	return nil
}
