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

	"github.com/spf13/viper"
	"gitlab.com/Shadow53/zip-builder/dl"
	"gitlab.com/Shadow53/zip-builder/lib"
)

// Only extracts libraries if being installed under /system
// To be fair, that's almost always where apps get installed...
func unzipSystemLibs(root string, zipinfo *lib.ZipInfo, app lib.AppInfo, ver, arch string, files *lib.Files) error {
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
				fileName := file.Name[strings.LastIndex(file.Name, "lib/")+4:]
				var wg sync.WaitGroup
				ch := make(chan string)
				wg.Add(len(lib.Arches))
				for _, a := range lib.Arches {
					go func(file *zip.File, ver, arch, a string, files *lib.Files, zipinfo *lib.ZipInfo, wg *sync.WaitGroup, ch chan string) {
						defer wg.Done()
						if !app.Android.Version[ver].HasArchSpecificInfo || arch == a {
							// Only create the parent folder if there is a file to extract
							destFolder := filepath.Join(root, "files", ver, a, app.PackageName+"-lib")
							err = os.MkdirAll(destFolder, os.ModeDir|0755)
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
								fileId := app.PackageName + "-" + file.Name
								// If file extracted correctly without problems, add to list
								files.Mux.RLock()
								files.File[fileId] = &lib.AndroidVersions{}
								files.File[fileId].Mux.Lock()
								if files.File[fileId].Version == nil {
									files.File[fileId].Version = make(map[string]*lib.AndroidVersionInfo)
								}
								files.File[fileId].Mux.Unlock()
								files.Mux.RUnlock()

								for i := sort.SearchStrings(lib.Versions, ver); i < len(lib.Versions) && app.Android.Version[lib.Versions[i]].Base == ver; i++ {
									v := lib.Versions[i]
									lib.Debug("Adding lib to Android version " + v)

									files.Mux.RLock()
									files.File[fileId].Mux.Lock()
									if files.File[fileId].Version[v] == nil {
										files.File[fileId].Version[v] = &lib.AndroidVersionInfo{
											Base:                ver,
											Arch:                make(map[string]*lib.FileInfo),
											HasArchSpecificInfo: true}
									}
									files.File[fileId].Mux.Unlock()
									files.Mux.RUnlock()

									dest := "/system/lib"
									if strings.Index(a, "64") > -1 {
										dest = dest + "64"
									}
									// Includes the '/'
									dest = dest + fileName[strings.LastIndex(fileName, "/"):]

									files.Mux.RLock()
									files.File[fileId].Mux.RLock()
									files.File[fileId].Version[v].Mux.Lock()
									files.File[fileId].Version[v].Arch[a] = &lib.FileInfo{
										Destination: dest,
										Mode:        "0644",
										FileName:    ver + "/" + a + "/" + app.PackageName + "-lib/" + fileName}
									files.File[fileId].Version[v].Mux.Unlock()
									files.File[fileId].Mux.RUnlock()
									files.Mux.RUnlock()
								}

								zipinfo.Mux.Lock()
								zipinfo.Files = append(zipinfo.Files, fileId)
								zipinfo.Mux.Unlock()
							}
						}
					}(file, ver, arch, a, files, zipinfo, &wg, ch)
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
func MakeZip(zip *lib.ZipInfo, apps *lib.Apps, files *lib.Files, ch chan error) {
	zip.Mux.RLock()
	lib.Debug("BUILDING ZIP: " + zip.Name)
	zippath := filepath.Join(viper.GetString("tempdir"), "build", zip.Name)
	zip.Mux.RUnlock()
	// Build zip root with files subdir
	err := os.MkdirAll(filepath.Join(zippath, "files"), os.ModeDir|0755)
	if err != nil {
		ch <- fmt.Errorf("Error while creating directory: %v\n  %v", filepath.Join(zippath, "files"), err)
		return
	}
	//defer os.RemoveAll(zippath)

	// Download apps
	zip.Mux.RLock()
	zipApps := zip.Apps
	zip.Mux.RUnlock()
	for _, app := range zipApps {
		lib.Debug("CHECKING TO DOWNLOAD " + app)
		for _, ver := range lib.Versions {
			appVer := ""
			apps.Mux.RLock()
			apps.App[app].Mux.RLock()
			apps.App[app].Android.Mux.RLock()
			if apps.App[app].Android.Version[ver] != nil {
				apps.App[app].Android.Version[ver].Mux.RLock()
				appVer = apps.App[app].Android.Version[ver].Base
				apps.App[app].Android.Version[ver].Mux.RUnlock()
			}
			apps.App[app].Android.Mux.RUnlock()
			apps.App[app].Mux.RUnlock()
			apps.Mux.RUnlock()
			if appVer == ver {
				for _, arch := range lib.Arches {
					apps.Mux.RLock()
					apps.App[app].Mux.RLock()
					apps.App[app].Android.Mux.RLock()
					apps.App[app].Android.Version[ver].Mux.RLock()
					apps.App[app].Android.Version[ver].Arch[arch].Mux.Lock()
					if apps.App[app].Android.Version[ver].Arch[arch].Url != "" {
						// Build file name with version/architecture
						filename := apps.App[app].PackageName + "-" + apps.App[app].Android.Version[ver].Base
						if apps.App[app].Android.Version[ver].HasArchSpecificInfo {
							filename = filename + "-" + arch
						}
						filename = filename + ".apk"
						lib.Debug("APP " + app + " FOR " + ver + " " + arch + " WILL BE SAVED AS " + filename)
						// Set file name for app
						apps.App[app].Android.Version[ver].Arch[arch].FileName = filename
						apppath := filepath.Join(zippath, "files", filename)
						if apps.App[app].UrlIsFDroidRepo {
							// Create separate variable to get around not being able to address map items
							err = dl.DownloadFromFDroidRepo(apps.App[app], ver, arch, apppath)
							if err != nil {
								apps.App[app].Android.Version[ver].Arch[arch].Mux.Unlock()
								apps.App[app].Android.Version[ver].Mux.RUnlock()
								apps.App[app].Mux.RUnlock()
								apps.Mux.RUnlock()
								ch <- err
								return
							}
						} else {
							err = dl.Download(apps.App[app].Android.Version[ver].Arch[arch].Url, apppath)
							if err != nil {
								apps.App[app].Android.Version[ver].Arch[arch].Mux.Unlock()
								apps.App[app].Android.Version[ver].Mux.RUnlock()
								apps.App[app].Mux.RUnlock()
								apps.Mux.RUnlock()
								ch <- fmt.Errorf("Error while downloading %v to %v:\n  %v",
									apps.App[app].Android.Version[ver].Arch[arch].Url, apppath, err)
								return
							}
						}
						// Test checksums
						md5sum := apps.App[app].Android.Version[ver].Arch[arch].MD5
						sha1sum := apps.App[app].Android.Version[ver].Arch[arch].SHA1
						sha256sum := apps.App[app].Android.Version[ver].Arch[arch].SHA256
						if md5sum != "" {
							fmt.Print("Checking md5sum... ")
							sum, err := lib.GetHash(apppath, "md5")
							if err != nil {
								apps.App[app].Android.Version[ver].Arch[arch].Mux.Unlock()
								apps.App[app].Android.Version[ver].Mux.RUnlock()
								apps.App[app].Mux.RUnlock()
								apps.Mux.RUnlock()
								ch <- fmt.Errorf("Error while calculating md5sum of %v:\n  %v", apppath, err)
								return
							}
							if sum != md5sum {
								apps.App[app].Android.Version[ver].Arch[arch].Mux.Unlock()
								apps.App[app].Android.Version[ver].Mux.RUnlock()
								apps.App[app].Mux.RUnlock()
								apps.Mux.RUnlock()
								ch <- fmt.Errorf("Unexpected md5sum. Expected %v but got %v", md5sum, sum)
								return
							}
							fmt.Println("md5sum matches")
						}
						if sha1sum != "" {
							fmt.Print("Checking sha1sum... ")
							sum, err := lib.GetHash(apppath, "sha1")
							if err != nil {
								apps.App[app].Android.Version[ver].Arch[arch].Mux.Unlock()
								apps.App[app].Android.Version[ver].Mux.RUnlock()
								apps.App[app].Mux.RUnlock()
								apps.Mux.RUnlock()
								ch <- fmt.Errorf("Error while calculating sha1sum of %v:\n  %v", apppath, err)
								return
							}
							if sum != sha1sum {
								apps.App[app].Android.Version[ver].Arch[arch].Mux.Unlock()
								apps.App[app].Android.Version[ver].Mux.RUnlock()
								apps.App[app].Mux.RUnlock()
								apps.Mux.RUnlock()
								ch <- fmt.Errorf("Unexpected md5sum. Expected %v but got %v", sha1sum, sum)
								return
							}
							fmt.Println("sha1sum matches")
						}
						if sha256sum != "" {
							fmt.Print("Checking sha256sum... ")
							sum, err := lib.GetHash(apppath, "sha256")
							if err != nil {
								apps.App[app].Android.Version[ver].Arch[arch].Mux.Unlock()
								apps.App[app].Android.Version[ver].Mux.RUnlock()
								apps.App[app].Mux.RUnlock()
								apps.Mux.RUnlock()
								ch <- fmt.Errorf("Error while calculating sha256sum of %v:\n  %v", apppath, err)
								return
							}
							if sum != sha256sum {
								apps.App[app].Android.Version[ver].Arch[arch].Mux.Unlock()
								apps.App[app].Android.Version[ver].Mux.RUnlock()
								apps.App[app].Mux.RUnlock()
								apps.Mux.RUnlock()
								ch <- fmt.Errorf("Unexpected sha256sum. Expected %v but got %v", sha256sum, sum)
								return
							}
							fmt.Println("sha256sum matches")
						}
						err := unzipSystemLibs(zippath, zip, *apps.App[app], ver, arch, files)
						if err != nil {
							apps.App[app].Android.Version[ver].Arch[arch].Mux.Unlock()
							apps.App[app].Android.Version[ver].Mux.RUnlock()
							apps.App[app].Mux.RUnlock()
							apps.Mux.RUnlock()
							ch <- fmt.Errorf("Error while unzipping libs from %v:\n  %v", apps.App[app].PackageName, err)
							return
						}
					} else {
						lib.Debug("WARNING: URL IS EMPTY FOR: " + app)
					}

					hasArchInfo := apps.App[app].Android.Version[ver].HasArchSpecificInfo
					apps.App[app].Android.Version[ver].Arch[arch].Mux.Unlock()
					apps.App[app].Android.Version[ver].Mux.RUnlock()
					apps.App[app].Mux.RUnlock()
					apps.Mux.RUnlock()

					// If there is no arch-specific info, the first is the same as the others. Break.
					if !hasArchInfo {
						break
					}
				}
			}
		}
	}

	// Download other files
	zip.Mux.RLock()
	zipFiles := zip.Files
	zip.Mux.RUnlock()
	for _, file := range zipFiles {
		lib.Debug("CHECKING TO DOWNLOAD " + file)
		for _, ver := range lib.Versions {
			fileVer := ""
			files.Mux.RLock()
			files.File[file].Mux.RLock()
			if files.File[file].Version[ver] != nil {
				files.File[file].Version[ver].Mux.RLock()
				fileVer = files.File[file].Version[ver].Base
				files.File[file].Version[ver].Mux.RUnlock()
			}
			files.File[file].Mux.RUnlock()
			files.Mux.RUnlock()

			if fileVer == ver {
				for _, arch := range lib.Arches {
					files.Mux.RLock()
					files.File[file].Mux.RLock()
					files.File[file].Version[ver].Mux.RLock()
					if files.File[file].Version[ver].Arch[arch] != nil {
						files.File[file].Version[ver].Arch[arch].Mux.Lock()
						if files.File[file].Version[ver].Arch[arch].Url != "" {
							filename := files.File[file].Version[ver].Arch[arch].FileName + "." + files.File[file].Version[ver].Base
							if files.File[file].Version[ver].HasArchSpecificInfo {
								filename = filename + "." + arch
							}
							lib.Debug("FILE " + file + " FOR " + ver + " " + arch + " WILL BE SAVED AS " + filename)
							files.File[file].Version[ver].Arch[arch].FileName = filename
							filepath := filepath.Join(zippath, "files", filename)
							err := dl.Download(files.File[file].Version[ver].Arch[arch].Url, filepath)
							if err != nil {
								files.File[file].Version[ver].Arch[arch].Mux.Unlock()
								files.File[file].Version[ver].Mux.RUnlock()
								files.File[file].Mux.RUnlock()
								files.Mux.RUnlock()
								ch <- fmt.Errorf("Error while downloading %v:\n  %v", files.File[file].Version[ver].Arch[arch].Url, err)
								return
							}
							// Test checksums
							md5sum := files.File[file].Version[ver].Arch[arch].MD5
							sha1sum := files.File[file].Version[ver].Arch[arch].SHA1
							sha256sum := files.File[file].Version[ver].Arch[arch].SHA256
							if md5sum != "" {
								fmt.Print("Checking md5sum... ")
								sum, err := lib.GetHash(filepath, "md5")
								if err != nil {
									files.File[file].Version[ver].Arch[arch].Mux.Unlock()
									files.File[file].Version[ver].Mux.RUnlock()
									files.File[file].Mux.RUnlock()
									files.Mux.RUnlock()
									ch <- fmt.Errorf("Error while calculating md5sum of %v:\n  %v", filepath, err)
									return
								}
								if sum != md5sum {
									files.File[file].Version[ver].Arch[arch].Mux.Unlock()
									files.File[file].Version[ver].Mux.RUnlock()
									files.File[file].Mux.RUnlock()
									files.Mux.RUnlock()
									ch <- fmt.Errorf("Unexpected md5sum. Expected %v but got %v", md5sum, sum)
									return
								}
								fmt.Println("md5sum matches")
							}
							if sha1sum != "" {
								fmt.Print("Checking sha1sum... ")
								sum, err := lib.GetHash(filepath, "sha1")
								if err != nil {
									files.File[file].Version[ver].Arch[arch].Mux.Unlock()
									files.File[file].Version[ver].Mux.RUnlock()
									files.File[file].Mux.RUnlock()
									files.Mux.RUnlock()
									ch <- fmt.Errorf("Error while calculating sha1sum of %v:\n  %v", filepath, err)
									return
								}
								if sum != sha1sum {
									files.File[file].Version[ver].Arch[arch].Mux.Unlock()
									files.File[file].Version[ver].Mux.RUnlock()
									files.File[file].Mux.RUnlock()
									files.Mux.RUnlock()
									ch <- fmt.Errorf("Unexpected sha1sum. Expected %v but got %v", sha1sum, sum)
									return
								}
								fmt.Println("sha1sum matches")
							}
							if sha256sum != "" {
								fmt.Print("Checking sha256sum... ")
								sum, err := lib.GetHash(filepath, "sha256")
								if err != nil {
									files.File[file].Version[ver].Arch[arch].Mux.Unlock()
									files.File[file].Version[ver].Mux.RUnlock()
									files.File[file].Mux.RUnlock()
									files.Mux.RUnlock()
									ch <- fmt.Errorf("Error while calculating sha256sum of %v:\n  %v", filepath, err)
									return
								}
								if sum != sha256sum {
									files.File[file].Version[ver].Arch[arch].Mux.Unlock()
									files.File[file].Version[ver].Mux.RUnlock()
									files.File[file].Mux.RUnlock()
									files.Mux.RUnlock()
									ch <- fmt.Errorf("Unexpected sha256sum. Expected %v but got %v", sha256sum, sum)
									return
								}
								fmt.Println("sha256sum matches")
							}
						} else {
							lib.Debug("WARNING: URL IS EMPTY FOR " + file)
						}
						files.File[file].Version[ver].Arch[arch].Mux.Unlock()
					}
					hasArchInfo := files.File[file].Version[ver].HasArchSpecificInfo
					files.File[file].Version[ver].Mux.RUnlock()
					files.File[file].Mux.RUnlock()
					files.Mux.RUnlock()
					if !hasArchInfo {
						break
					}
				}

			}
		}
	}

	err = makePermsFile(zippath, zip, apps, files)
	if err != nil {
		ch <- fmt.Errorf("Error while creating permissions file:\n  %v", err)
		return
	}

	err = makeSysconfigFile(zippath, zip, apps, files)
	if err != nil {
		ch <- fmt.Errorf("Error while creating sysconfig file:\n  %v", err)
		return
	}

	err = makeAddondScripts(zippath, zip, apps, files)
	if err != nil {
		lib.Debug("ERROR GENERATING ADDON.D")
		ch <- fmt.Errorf("Error while creating addon.d survival script:\n  %v", err)
		return
	}

	err = makeUpdaterScript(zippath, zip, apps, files)
	if err != nil {
		ch <- fmt.Errorf("Error while creating updater-script:\n  %v", err)
		return
	}

	// Source is hardcoded because I know it will not change until I change it
	fmt.Println("Downloading update-binary")
	err = dl.Download("https://gitlab.com/Shadow53/zip-builder/raw/master/update-binary", filepath.Join(zippath, "META-INF", "com", "google", "android", "update-binary"))
	if err != nil {
		ch <- fmt.Errorf("Error while downloading update-binary from zip-builder repo:\n  %v", err)
		return
	}
	// Generate zip and md5 file
	zipLocation, err := zipFolder(zippath, *zip)
	if err != nil {
		ch <- fmt.Errorf("Error while zipping contents of %v:\n  %v", zippath, err)
		return
	}
	err = lib.GenerateMD5File(zipLocation)
	if err != nil {
		ch <- fmt.Errorf("Error while generating md5 for zip at %v:\n  %v", zipLocation, err)
		return
	}
}
