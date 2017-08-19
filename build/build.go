package build

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
	"gitlab.com/Shadow53/zip-builder/dl"
	"gitlab.com/Shadow53/zip-builder/lib"
)

// Only extracts libraries if being installed under /system
// To be fair, that's almost always where apps get installed...
func unzipSystemLibs(root string, zipinfo *lib.ZipInfo, app lib.AppInfo, ver, arch string, files *lib.Files) error {
	if strings.HasPrefix(app.AndroidVersion[ver].Arch[arch].Destination, "/system/") {
		// Hold all library files for this app in {ZIPROOT}/files/app-lib/
		destFolder := filepath.Join(root, "files", ver, arch, app.PackageName+"-lib")
		zipLoc := filepath.Join(root, "files", app.AndroidVersion[ver].Arch[arch].FileName)
		reader, err := zip.OpenReader(zipLoc)
		if err != nil {
			return fmt.Errorf("Error while opening the apk at %v:\n  %v", zipLoc, err)
		}

		// Extract only files whose paths begin with "lib/"
		for _, file := range reader.File {
			if strings.HasPrefix(file.Name, "lib/") {
				// Only create the parent folder if there is a file to extract
				err = os.MkdirAll(destFolder, os.ModeDir|0755)
				if err != nil {
					return fmt.Errorf("Error while making a directory at %v:\n  %v", destFolder, err)
				}

				fileName := file.Name[strings.LastIndex(file.Name, "lib/")+4:]
				for _, a := range lib.Arches {
					if !app.AndroidVersion[ver].HasArchSpecificInfo || arch == a {
						if (a == "arm" && strings.Index(fileName, "armeabi") > -1) || (a != "arm" && strings.Index(fileName, a) > -1) {
							path := filepath.Join(destFolder, fileName)
							err = os.MkdirAll(path[:strings.LastIndex(path, "/")], os.ModeDir|0755)
							if err != nil {
								return fmt.Errorf("Error while making a directory at %v:\n  %v", path[:strings.LastIndex(path, "/")], err)
							}

							fileReader, err := file.Open()
							if err != nil {
								return fmt.Errorf("Error while opening %v from apk for reading:\n  %v", file.Name, err)
							}
							defer fileReader.Close()

							targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
							if err != nil {
								return fmt.Errorf("Error while opening %v for writing:\n  %v", path, err)
							}
							defer targetFile.Close()

							fmt.Println("Extracting library from " + app.PackageName + ": " + file.Name)
							_, err = io.Copy(targetFile, fileReader)
							if err != nil {
								return fmt.Errorf("Error while copying from %v to %v:\n  %v", file.Name, path, err)
							}

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

								dest := "/system/lib"
								if strings.Index(a, "64") > -1 {
									dest = dest + "64"
								}
								// Includes the '/'
								dest = dest + fileName[strings.LastIndex(fileName, "/"):]

								(*files)[fileId][v].Arch[a] = lib.FileInfo{
									Destination: dest,
									Mode:        "0644",
									FileName:    ver + "/" + arch + "/" + app.PackageName + "-lib/" + fileName}
							}

							zipinfo.Files = append(zipinfo.Files, fileId)
						}
					}
				}
			}
		}
	}
	return nil
}

func zipFolder(root string, zipinfo lib.ZipInfo) (string, error) {
	zipdest := filepath.Join(viper.GetString("destination"), zipinfo.Name+".zip")
	fmt.Println("Creating zip file at " + zipdest)
	// Create destination directory if it doesn't exist
	err := os.MkdirAll(viper.GetString("destination"), os.ModeDir|0755)
	if err != nil {
		return "", fmt.Errorf("Error while making directory %v:\n  %v", viper.GetString("destination"), err)
	}

	zipfile, err := os.Create(zipdest)
	if err != nil {
		return "", fmt.Errorf("Error while creating target zip file %v:\n  %v", zipdest, err)
	}

	defer fmt.Println("Zip file created")
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("Error while zipping file %v into %v:\n  %v", path, zipdest, err)
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("Error while generating FileInfoHeader for %v:\n  %v", path, err)
		}

		header.Name = strings.TrimPrefix(path, root)
		header.Name = strings.TrimPrefix(header.Name, "/")

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("Error while creating file header for %v inside %v:\n  %v", path, zipdest, err)
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("Error while opening %v for reading:\n  %v", path, err)
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		if err != nil {
			return fmt.Errorf("Error while archiving %v:\n  %v", path, err)
		}
		return nil
	})
	return zipdest, nil
}

// TODO: Change app dl-ing to error if app doesn't exist
func MakeZip(zip lib.ZipInfo, apps lib.Apps, files lib.Files) error {
	lib.Debug("BUILDING ZIP: " + zip.Name)
	zippath := filepath.Join(viper.GetString("tempdir"), "build", zip.Name)
	// Build zip root with files subdir
	err := os.MkdirAll(filepath.Join(zippath, "files"), os.ModeDir|0755)
	if err != nil {
		return fmt.Errorf("Error while creating directory", filepath.Join(zippath, "files"), err)
	}
	defer os.RemoveAll(zippath)

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
							err = dl.DownloadFromFDroidRepo(&appRef, ver, arch, apppath)
							if err != nil {
								return err
							}
							apps[app] = appRef
						} else {
							err = dl.Download(apps[app].AndroidVersion[ver].Arch[arch].Url, apppath)
							if err != nil {
								return fmt.Errorf("Error while downloading %v to %v:\n  %v",
									apps[app].AndroidVersion[ver].Arch[arch].Url, apppath, err)
							}
						}
						// Test checksums
						md5sum := apps[app].AndroidVersion[ver].Arch[arch].MD5
						sha1sum := apps[app].AndroidVersion[ver].Arch[arch].SHA1
						sha256sum := apps[app].AndroidVersion[ver].Arch[arch].SHA256
						if md5sum != "" {
							fmt.Print("Checking md5sum... ")
							sum, err := lib.GetHash(apppath, "md5")
							if err != nil {
								return fmt.Errorf("Error while calculating md5sum of %v:\n  %v", apppath, err)
							}
							if sum != md5sum {
								return fmt.Errorf("Unexpected md5sum. Expected %v but got %v", md5sum, sum)
							}
							fmt.Println("md5sum matches")
						}
						if sha1sum != "" {
							fmt.Print("Checking sha1sum... ")
							sum, err := lib.GetHash(apppath, "sha1")
							if err != nil {
								return fmt.Errorf("Error while calculating sha1sum of %v:\n  %v", apppath, err)
							}
							if sum != sha1sum {
								return fmt.Errorf("Unexpected md5sum. Expected %v but got %v", sha1sum, sum)
							}
							fmt.Println("sha1sum matches")
						}
						if sha256sum != "" {
							fmt.Print("Checking sha256sum... ")
							sum, err := lib.GetHash(apppath, "sha256")
							if err != nil {
								return fmt.Errorf("Error while calculating sha256sum of %v:\n  %v", apppath, err)
							}
							if sum != sha256sum {
								return fmt.Errorf("Unexpected sha256sum. Expected %v but got %v", sha256sum, sum)
							}
							fmt.Println("sha256sum matches")
						}
						err := unzipSystemLibs(zippath, &zip, apps[app], ver, arch, &files)
						if err != nil {
							return fmt.Errorf("Error while unzipping libs from %v:\n  %v", apps[app].PackageName, err)
						}
					} else {
						lib.Debug("WARNING: URL IS EMPTY FOR: " + app)
					}
					// If there is no arch-specific info, the first is the same as the others. Break.
					if !apps[app].AndroidVersion[ver].HasArchSpecificInfo {
						break
					}
				}
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
						err := dl.Download(files[file][ver].Arch[arch].Url, filepath)
						if err != nil {
							return fmt.Errorf("Error while downloading %v:\n  %v", files[file][ver].Arch[arch].Url, err)
						}
						// Test checksums
						md5sum := files[file][ver].Arch[arch].MD5
						sha1sum := files[file][ver].Arch[arch].SHA1
						sha256sum := files[file][ver].Arch[arch].SHA256
						if md5sum != "" {
							fmt.Print("Checking md5sum... ")
							sum, err := lib.GetHash(filepath, "md5")
							if err != nil {
								return fmt.Errorf("Error while calculating md5sum of %v:\n  %v", filepath, err)
							}
							if sum != md5sum {
								return fmt.Errorf("Unexpected md5sum. Expected %v but got %v", md5sum, sum)
							}
							fmt.Println("md5sum matches")
						}
						if sha1sum != "" {
							fmt.Print("Checking sha1sum... ")
							sum, err := lib.GetHash(filepath, "sha1")
							if err != nil {
								return fmt.Errorf("Error while calculating sha1sum of %v:\n  %v", filepath, err)
							}
							if sum != sha1sum {
								return fmt.Errorf("Unexpected sha1sum. Expected %v but got %v", sha1sum, sum)
							}
							fmt.Println("sha1sum matches")
						}
						if sha256sum != "" {
							fmt.Print("Checking sha256sum... ")
							sum, err := lib.GetHash(filepath, "sha256")
							if err != nil {
								return fmt.Errorf("Error while calculating sha256sum of %v:\n  %v", filepath, err)
							}
							if sum != sha256sum {
								return fmt.Errorf("Unexpected sha256sum. Expected %v but got %v", sha256sum, sum)
							}
							fmt.Println("sha256sum matches")
						}
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

	err = makePermsFile(zippath, &zip, apps, &files)
	if err != nil {
		return fmt.Errorf("Error while creating permissions file:\n  %v", err)
	}

	err = makeSysconfigFile(zippath, &zip, apps, &files)
	if err != nil {
		return fmt.Errorf("Error while creating sysconfig file:\n  %v", err)
	}

	err = makeAddondScripts(zippath, &zip, apps, &files)
	if err != nil {
		return fmt.Errorf("Error while creating addon.d survival script:\n  %v", err)
	}

	err = makeUpdaterScript(zippath, zip, apps, files)
	if err != nil {
		return fmt.Errorf("Error while creating updater-script:\n  %v", err)
	}

	// Source is hardcoded because I know it will not change until I change it
	fmt.Println("Downloading update-binary")
	err = dl.Download("https://gitlab.com/Shadow53/zip-builder/raw/master/update-binary", filepath.Join(zippath, "META-INF", "com", "google", "android", "update-binary"))
	if err != nil {
		return fmt.Errorf("Error while downloading update-binary from zip-builder repo:\n  %v", err)
	}
	// Generate zip and md5 file
	zipLocation, err := zipFolder(zippath, zip)
	if err != nil {
		return fmt.Errorf("Error while zipping contents of %v:\n  %v", zippath, err)
	}
	err = lib.GenerateMD5File(zipLocation)
	if err != nil {
		return fmt.Errorf("Error while generating md5 for zip at %v:\n  %v", zipLocation, err)
	}
	return nil
}
