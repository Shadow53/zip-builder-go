package build

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gitlab.com/Shadow53/zip-builder/lib"
)

func processUnzipFile(file *zip.File, app *lib.AppInfo, root, ver, arch, a string, files *lib.Files, zipinfo *lib.ZipInfo, wg *sync.WaitGroup, ch chan string) {
	defer wg.Done()
	if !app.Android.Version[ver].HasArchSpecificInfo || arch == a {
		// Only create the parent folder if there is a file to extract
		fileName := file.Name[strings.LastIndex(file.Name, "lib/")+4:]
		destFolder := filepath.Join(root, "files", app.PackageName+"-lib", ver)
		err := os.MkdirAll(destFolder, os.ModeDir|0755)
		if err != nil {
			ch <- fmt.Sprintf("Error while making a directory at %v:\n  %v", destFolder, err)
			return
		}
		if (a == "arm" && strings.Index(fileName, "armeabi") > -1) || (a != "arm" && strings.Index(fileName, a) > -1 && (a != "x86" || strings.Index(fileName, "x86_64") == -1)) {
			path := filepath.Join(destFolder, fileName)
			err = os.MkdirAll(path[:strings.LastIndex(path, "/")], os.ModeDir|0755)
			if err != nil {
				ch <- fmt.Sprintf("Error while making a directory at %v:\n  %v", path[:strings.LastIndex(path, "/")], err)
				return
			}

			fileReader, err := file.Open()
			if err != nil {
				ch <- fmt.Sprintf("Error while opening %v from apk for reading:\n  %v", file.Name, err)
				return
			}
			defer fileReader.Close()

			targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				ch <- fmt.Sprintf("Error while opening %v for writing:\n  %v", path, err)
				return
			}
			defer targetFile.Close()

			fmt.Println("Extracting library from " + app.PackageName + ": " + file.Name)
			_, err = io.Copy(targetFile, fileReader)
			if err != nil {
				ch <- fmt.Sprintf("Error while copying from %v to %v:\n  %v", file.Name, path, err)
				return
			}

			lib.Debug("Creating file entry for " + file.Name)
			// Will only be reached by actual files
			zipinfo.RLock()
			fileId := zipinfo.Name + "-" + app.PackageName + "-" + file.Name
			zipinfo.RUnlock()
			// If file extracted correctly without problems, add to list
			files.Lock()
			files.File[fileId] = &lib.AndroidVersions{}
			files.Unlock()

			files.LockFile(fileId)
			if files.GetFile(fileId).Version == nil {
				files.GetFile(fileId).Version = make(map[string]*lib.AndroidVersionInfo)
			}
			files.UnlockFile(fileId)

			for i := sort.SearchStrings(zipinfo.Versions, ver); i < len(zipinfo.Versions) && app.Android.Version[zipinfo.Versions[i]].Base == ver; i++ {
				v := zipinfo.Versions[i]
				lib.Debug("Adding lib to Android version " + v)

				files.LockFile(fileId)
				if !files.FileVersionExists(fileId, v) {
					files.SetFileVersion(fileId, v, &lib.AndroidVersionInfo{
						Base:                ver,
						Arch:                make(map[string]*lib.FileInfo),
						HasArchSpecificInfo: true})
				}
				files.UnlockFile(fileId)

				dest := app.Android.Version[ver].Arch[arch].Destination //"/system/lib"
				dest = dest[0:strings.LastIndex(dest, "/")+1] + "lib/" + fileName
				/*if strings.Index(a, "64") > -1 {
					dest = dest + "64"
				}
				// Includes the '/'
				dest = dest + fileName[strings.LastIndex(fileName, "/"):]*/

				files.LockFileVersion(fileId, v)
				files.SetFileVersionArch(fileId, v, a, &lib.FileInfo{
					Destination: dest,
					Mode:        "0644",
					FileName:    app.PackageName + "-lib/" + ver + "/" + fileName})
				files.UnlockFileVersion(fileId, v)
			}

			zipinfo.Lock()
			zipinfo.Files = append(zipinfo.Files, fileId)
			zipinfo.Unlock()
		}
	}
}

// Only extracts libraries if being installed under /system
// To be fair, that's almost always where apps get installed...
func unzipSystemLibs(root string, zipinfo *lib.ZipInfo, app *lib.AppInfo, ver, arch string, files *lib.Files) error {
	if strings.HasPrefix(app.Android.Version[ver].Arch[arch].Destination, "/system/") {
		// Hold all library files for this app in {ZIPROOT}/files/app-lib/
		zipLoc := filepath.Join(root, "files", app.Android.Version[ver].Arch[arch].FileName)
		reader, err := zip.OpenReader(zipLoc)
		if err != nil {
			return fmt.Errorf("Error while opening the apk at %v:\n  %v", zipLoc, err)
		}

		// Extract only files whose paths begin with "lib/"
		for _, file := range reader.File {
			if strings.HasPrefix(file.Name, "lib/") {
				var wg sync.WaitGroup
				ch := make(chan string)
				wg.Add(len(zipinfo.Arches))
				for _, a := range zipinfo.Arches {
					go processUnzipFile(file, app, root, ver, arch, a, files, zipinfo, &wg, ch)
				}
				wg.Wait()
				close(ch)
				errs := ""
				for err := range ch {
					errs = errs + err + "\n  "
				}
				if errs != "" {
					return fmt.Errorf(errs)
				}
			}
		}
	}
	return nil
}
