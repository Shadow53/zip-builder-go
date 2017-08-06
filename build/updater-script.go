package build

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/Shadow53/lib"
)

// TODO: Add support for arch-specific and Android version-specific files
func makeFileInstallScriptlet(file lib.FileInfo, buffer *bytes.Buffer) {
	// Create the parent directories of the file and set their metadata
	destParent := file.Destination[0:strings.LastIndex(file.Destination, "/")]
	buffer.WriteString("assert(run_program(\"/sbin/busybox\", \"mkdir\", \"-p\", \"")
	buffer.WriteString(destParent)
	buffer.WriteString("\") == 0);\n")
	buffer.WriteString("assert(set_metadata_recursive(\"")
	buffer.WriteString(destParent)
	buffer.WriteString("\", \"uid\", 0, \"gid\", 0, \"fmode\", 0644, \"dmode\", 0755) == \"\");\n")
	// Tell the user what is happening
	buffer.WriteString("ui_print(\"Extracting ")
	buffer.WriteString(file.Destination)
	buffer.WriteString("\");\n")
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

func makeFileDeleteScriptlet(filesToDelete map[string]bool, buffer *bytes.Buffer) {
	for file, _ := range filesToDelete {
		// The weird spacing should cause a nice tree structure in the output
		// The generated code should recursively delete directories and normal delete files
		// TODO: Add output telling what is happening
		buffer.WriteString("ifelse(run_program(\"/sbin/busybox\", \"test\", \"-d\", \"")
		buffer.WriteString(file)
		buffer.WriteString("\") == 0, \n    ui_print(\"Recursively deleting existing folder ")
		buffer.WriteString(file)
		buffer.WriteString("\") && delete_recursive(\"")
		buffer.WriteString(file)
		buffer.WriteString("\"),\n    ifelse(run_program(\"/sbin/busybox\", \"test\", \"-f\", \"")
		buffer.WriteString(file)
		buffer.WriteString("\") == 0,\n        ui_print(\"Deleting existing file ")
		buffer.WriteString(file)
		buffer.WriteString("\") && delete(\"")
		buffer.WriteString(file)
		buffer.WriteString("\")\n    )\n);\n")
	}
}

func makePerItemScriptlet(item map[string]lib.AndroidVersionInfo, buff *bytes.Buffer) {
	multVersionTest := ""
	verFilesToDelete := make(map[string]bool)
	var archBuff bytes.Buffer
	var extractFiles bytes.Buffer
	var deleteFiles bytes.Buffer
	for i, ver := range lib.Versions {
		lib.Debug("ANDROID VERSION: " + ver)
		testVersion := "is_substring(\"" + ver + "\", getprop(\"ro.build.version.release\"))"
		if item[ver].Base != "" {
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
						if item[ver].Arch[arch].FileName != "" {
							lib.Debug("TESTING FOR ANDROID ARCH: " + arch)
							archBuff.WriteString("if is_substring(\"")
							if arch == "arm" {
								archBuff.WriteString("armeabi")
							} else {
								archBuff.WriteString(arch)
							}
							archBuff.WriteString("\", getprop(\"ro.product.cpu.abilist\") + getprop(\"ro.product.cpu.abi\")) then\n")

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

func makeUpdaterScript(root string, zip lib.ZipInfo, apps lib.Apps, files lib.Files) {
	log.Println("Generating updater-script")

	var script bytes.Buffer

	// TODO: Dynamically decide which partitions need to be mounted
	script.WriteString(`ui_print("--------------------------------------");
ui_print("Mounting system");
ifelse(is_mounted("/system"), unmount("/system"));
run_program("/sbin/busybox", "mount", "/system");
`)

	filesToDelete := make(map[string]bool)
	for _, del := range zip.InstallRemoveFiles {
		filesToDelete[del] = true
	}
	makeFileDeleteScriptlet(filesToDelete, &script)

	for _, app := range zip.Apps {
		if apps[app].PackageName != "" {
			makePerItemScriptlet(apps[app].AndroidVersion, &script)
		}
	}

	for _, file := range zip.Files {
		makePerItemScriptlet(files[file], &script)
	}

	script.WriteString(`ui_print("Unmounting /system");
ui_print("Done!");
ui_print("--------------------------------------");
unmount("/system");
`)

	scriptDest := filepath.Join(root, "/META-INF/com/google/android")
	os.MkdirAll(scriptDest, os.ModeDir|0755)
	scriptDest = filepath.Join(scriptDest, "updater-script")

	ioutil.WriteFile(scriptDest, []byte(script.String()), 0644)
}
