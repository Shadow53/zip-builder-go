package build

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/viper"
	"gitlab.com/Shadow53/zip-builder/dl"
	"gitlab.com/Shadow53/zip-builder/lib"
)

func checkChecksums(file *lib.FileInfo, path string) error {
	md5sum := file.MD5
	sha1sum := file.SHA1
	sha256sum := file.SHA256
	if md5sum != "" {
		fmt.Print("Checking md5sum... ")
		sum, err := lib.GetHash(path, "md5")
		if err != nil {
			return fmt.Errorf("Error while calculating md5sum of %v:\n  %v", path, err)
		}
		if sum != md5sum {
			lib.Debug("Wrong MD5SUM")
			return fmt.Errorf("Unexpected md5sum. Expected %v but got %v", md5sum, sum)
		}
		fmt.Println("md5sum matches")
	}
	if sha1sum != "" {
		fmt.Print("Checking sha1sum... ")
		sum, err := lib.GetHash(path, "sha1")
		if err != nil {
			return fmt.Errorf("Error while calculating sha1sum of %v:\n  %v", path, err)
		}
		if sum != sha1sum {
			return fmt.Errorf("Unexpected md5sum. Expected %v but got %v", sha1sum, sum)
		}
		fmt.Println("sha1sum matches")
	}
	if sha256sum != "" {
		fmt.Print("Checking sha256sum... ")
		sum, err := lib.GetHash(path, "sha256")
		if err != nil {
			return fmt.Errorf("Error while calculating sha256sum of %v:\n  %v", path, err)
		}
		if sum != sha256sum {
			return fmt.Errorf("Unexpected sha256sum. Expected %v but got %v", sha256sum, sum)
		}
		fmt.Println("sha256sum matches")
	}
	return nil
}

// TODO: Change app dl-ing to error if app doesn't exist
func MakeZip(zip *lib.ZipInfo, apps *lib.Apps, files *lib.Files, ch chan error) {
	zip.RLock()
	lib.Debug("BUILDING ZIP: " + zip.Name)
	zippath := filepath.Join(viper.GetString("tempdir"), "build", zip.Name)
	zip.RUnlock()
	// Build zip root with files subdir
	err := os.MkdirAll(filepath.Join(zippath, "files"), os.ModeDir|0755)
	if err != nil {
		ch <- fmt.Errorf("Error while creating directory: %v\n  %v", filepath.Join(zippath, "files"), err)
		return
	}
	//defer os.RemoveAll(zippath)

	// Download apps
	zip.RLock()
	zipApps := zip.Apps
	zipFiles := zip.Files
	zip.RUnlock()

	var zipwg, errwg sync.WaitGroup
	cherr := make(chan error)
	doBuild := true

	// Don't continue if an error occurred
	go func(receive, send chan error, doBuild *bool, wg *sync.WaitGroup) {
		wg.Add(1)
		for err := range receive {
			send <- err
			*doBuild = false
		}
		wg.Done()
	}(cherr, ch, &doBuild, &errwg)

	for _, app := range zipApps {
		lib.Debug("CHECKING TO DOWNLOAD " + app)
		for _, ver := range lib.Versions {
			appVer := ""
			hasArchInfo := false
			apps.RLockApp(app)
			if apps.AppVersionExists(app, ver) {
				apps.RLockAppVersion(app, ver)
				appVer = apps.GetAppVersion(app, ver).Base
				hasArchInfo = apps.GetAppVersion(app, ver).HasArchSpecificInfo
				apps.RUnlockAppVersion(app, ver)
			}
			apps.RUnlockApp(app)

			if appVer == ver {
				if hasArchInfo {
					zipwg.Add(len(lib.Arches))
					lib.Debug("Added download for each arch")
					for _, arch := range lib.Arches {
						go DownloadApp(zip, files, apps, app, ver, arch, zippath, cherr, &zipwg)
					}
				} else {
					zipwg.Add(1)
					lib.Debug("Added one download")
					go DownloadApp(zip, files, apps, app, ver, lib.Arches[0], zippath, cherr, &zipwg)
				}
			}
		}
	}

	// Download other files
	for _, file := range zipFiles {
		lib.Debug("CHECKING TO DOWNLOAD " + file)
		for _, ver := range lib.Versions {
			fileVer := ""
			hasArchInfo := false
			files.RLockFile(file)
			if files.FileVersionExists(file, ver) {
				files.RLockFileVersion(file, ver)
				fileVer = files.GetFileVersion(file, ver).Base
				hasArchInfo = files.GetFileVersion(file, ver).HasArchSpecificInfo
				files.RUnlockFileVersion(file, ver)
			}
			files.RUnlockFile(file)

			if fileVer == ver {
				if hasArchInfo {
					zipwg.Add(len(lib.Arches))
					for _, arch := range lib.Arches {
						go DownloadFile(files, file, ver, arch, zippath, cherr, &zipwg)
					}
				} else {
					zipwg.Add(1)
					go DownloadFile(files, file, ver, lib.Arches[0], zippath, cherr, &zipwg)
				}
			}
		}
	}

	fmt.Println("Waiting for files and apps to finish downloading")
	zipwg.Wait()
	close(cherr)
	errwg.Wait()

	if !doBuild {
		zip.RLock()
		fmt.Println("Error(s) occurred while downloading apps/files for " + zip.Name)
		fmt.Println(zip.Name + " will not be built unless errors are resolved.")
		zip.RUnlock()
		return
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
	zipLocation, err := zipFolder(zippath, zip)
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
