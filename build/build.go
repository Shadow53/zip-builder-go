package build

import (
	"archive/zip"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
	"gitlab.com/Shadow53/dl"
	"gitlab.com/Shadow53/lib"
)

// Only extracts libraries if being installed under /system
// To be fair, that's almost always where apps get installed...
func unzipSystemLibs(root string, zipinfo *lib.ZipInfo, app lib.AppInfo, ver, arch string, files *lib.Files) {
	if strings.HasPrefix(app.AndroidVersion[ver].Arch[arch].Destination, "/system/") {
		// Hold all library files for this app in {ZIPROOT}/files/app-lib/
		destFolder := filepath.Join(root, "files", ver, arch, app.PackageName+"-lib")

		reader, err := zip.OpenReader(filepath.Join(root, "files", app.AndroidVersion[ver].Arch[arch].FileName))
		lib.ExitIfError(err)

		// Extract only files whose paths begin with "lib/"
		for _, file := range reader.File {
			if strings.HasPrefix(file.Name, "lib/") {
				// Only create the parent folder if there is a file to extract
				err = os.MkdirAll(destFolder, os.ModeDir|0755)
				lib.ExitIfError(err)

				fileName := file.Name[strings.LastIndex(file.Name, "lib/")+4:]
				for _, a := range lib.Arches {
					if !app.AndroidVersion[ver].HasArchSpecificInfo || arch == a {
						if (a == "arm" && strings.Index(fileName, "armeabi") > -1) || (a != "arm" && strings.Index(fileName, a) > -1) {
							path := filepath.Join(destFolder, fileName)
							os.MkdirAll(path[:strings.LastIndex(path, "/")], os.ModeDir|0755)

							fileReader, err := file.Open()
							lib.ExitIfError(err)
							defer fileReader.Close()

							targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
							lib.ExitIfError(err)
							defer targetFile.Close()

							log.Println("Extracting library from " + app.PackageName + ": " + file.Name)
							_, err = io.Copy(targetFile, fileReader)
							lib.ExitIfError(err)

							// Will only be reached by actual files
							fileId := app.PackageName + "-" + file.Name
							// If file extracted correctly without problems, add to list
							if (*files)[fileId] == nil {
								(*files)[fileId] = make(map[string]lib.AndroidVersionInfo)
							}

							for i := sort.SearchStrings(lib.Versions, ver); i < len(lib.Versions) && app.AndroidVersion[lib.Versions[i]].Base == ver; i++ {
								v := lib.Versions[i]
								if (*files)[fileId][v].Base == "" {
									(*files)[fileId][v] = lib.AndroidVersionInfo{
										Base:                ver,
										Arch:                make(map[string]lib.FileInfo),
										HasArchSpecificInfo: true}
								}

								(*files)[fileId][v].Arch[a] = lib.FileInfo{
									Destination: app.AndroidVersion[ver].Arch[arch].Destination[:strings.LastIndex(app.AndroidVersion[ver].Arch[arch].Destination, "/")] + "/" + file.Name,
									Mode:        "0644",
									FileName:    ver + "/" + a + "/" + app.PackageName + "-lib/" + fileName}
							}

							zipinfo.Files = append(zipinfo.Files, fileId)
						} else {
							lib.Debug("FILE AT " + file.Name + " DOES NOT MATCH ARCH " + a)
						}
					}
				}
			}
		}
	}
}

func zipFolder(root string, zipinfo lib.ZipInfo) string {
	zipdest := filepath.Join(viper.GetString("destination"), zipinfo.Name+".zip")
	log.Println("Creating zip file at " + zipdest)
	// Create destination directory if it doesn't exist
	os.MkdirAll(viper.GetString("destination"), os.ModeDir|0755)
	zipfile, err := os.Create(zipdest)
	lib.ExitIfError(err)
	defer log.Println("Zip file created")
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		lib.ExitIfError(err)

		header, err := zip.FileInfoHeader(info)
		lib.ExitIfError(err)

		header.Name = strings.TrimPrefix(path, root)
		header.Name = strings.TrimPrefix(header.Name, "/")

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		lib.ExitIfError(err)

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		lib.ExitIfError(err)
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})
	return zipdest
}

// TODO: Change app dl-ing to error if app doesn't exist
func MakeZip(zip lib.ZipInfo, apps lib.Apps, files lib.Files) {
	lib.Debug("BUILDING ZIP: " + zip.Name)
	zippath := filepath.Join(viper.GetString("tempdir"), "build", zip.Name)
	// Build zip root with files subdir
	err := os.MkdirAll(filepath.Join(zippath, "files"), os.ModeDir|0755)
	//defer os.RemoveAll(zippath)
	lib.ExitIfError(err)

	// Download apps
	for _, app := range zip.Apps {
		lib.Debug("CHECKING TO DOWNLOAD " + app)
		for _, ver := range lib.Versions {
			if apps[app].AndroidVersion[ver].Base == ver {
				for _, arch := range lib.Arches {
					if apps[app].AndroidVersion[ver].Arch[arch].Url != "" {
						// Build file name with version/architecture
						filename := apps[app].PackageName + "-" + apps[app].AndroidVersion[ver].Base
						if apps[app].AndroidVersion[ver].HasArchSpecificInfo {
							filename = filename + "-" + arch
						}
						filename = filename + ".apk"
						lib.Debug("APP " + app + " FOR " + ver + " " + arch + " WILL BE SAVED AS " + filename)
						// Set file name for app
						archInfo := apps[app].AndroidVersion[ver].Arch[arch]
						archInfo.FileName = filename
						apps[app].AndroidVersion[ver].Arch[arch] = archInfo
						apppath := filepath.Join(zippath, "files", filename)
						if apps[app].UrlIsFDroidRepo {
							// Create separate variable to get around not being able to address map items
							appRef := apps[app]
							dl.DownloadFromFDroidRepo(&appRef, ver, arch, apppath)
							apps[app] = appRef
						} else {
							dl.Download(apps[app].AndroidVersion[ver].Arch[arch].Url, apppath)
						}
						unzipSystemLibs(zippath, &zip, apps[app], ver, arch, &files)
						// TODO: Verify hash of file, error on mismatch
					} else {
						lib.Debug("WARNING: URL IS EMPTY FOR: " + app)
					}
					// If there is no arch-specific info, the first is the same as the others. Break.
					if !apps[app].AndroidVersion[ver].HasArchSpecificInfo {
						break
					}
				}
			} else {
				lib.Debug("WARNING: BASE ANDROID VERSION \"" + apps[app].AndroidVersion[ver].Base + " DOES NOT MATCH \"" + ver + "\"")
			}
		}
	}
	// Download other files
	for _, file := range zip.Files {
		lib.Debug("CHECKING TO DOWNLOAD " + file)
		for _, ver := range lib.Versions {
			if files[file][ver].Base == ver {
				for _, arch := range lib.Arches {
					if files[file][ver].Arch[arch].Url != "" {
						filename := files[file][ver].Arch[arch].FileName + "." + files[file][ver].Base
						if files[file][ver].HasArchSpecificInfo {
							filename = filename + "." + arch
						}
						lib.Debug("FILE " + file + " FOR " + ver + " " + arch + " WILL BE SAVED AS " + filename)
						archInfo := files[file][ver].Arch[arch]
						archInfo.FileName = filename
						files[file][ver].Arch[arch] = archInfo
						filepath := filepath.Join(zippath, "files", filename)
						dl.Download(files[file][ver].Arch[arch].Url, filepath)
						// TODO: Verify hash of file, error on mismatch
					} else {
						lib.Debug("WARNING: URL IS EMPTY FOR " + file)
					}
					if !files[file][ver].HasArchSpecificInfo {
						break
					}
				}
			}
		}
	}

	makePermsFile(zippath, &zip, apps, &files)
	makeSysconfigFile(zippath, &zip, apps, &files)
	makeAddondScripts(zippath, &zip, apps, &files)
	makeUpdaterScript(zippath, zip, apps, files)
	// Source is hardcoded because I know it will not change until I change it
	log.Println("Downloading update-binary")
	dl.Download("https://gitlab.com/Shadow53/zip-builder/raw/master/update-binary", filepath.Join(zippath, "META-INF", "com", "google", "android", "update-binary"))

	// Generate zip and md5 file
	lib.GenerateMD5File(zipFolder(zippath, zip))
}
