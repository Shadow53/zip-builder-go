package build

import (
	"fmt"
	"path/filepath"
	"sync"

	"gitlab.com/Shadow53/zip-builder/dl"
	"gitlab.com/Shadow53/zip-builder/lib"
)

func downloadApp(apps *lib.Apps, zip *lib.ZipInfo, app, ver, arch, apppath string) error {
	if apps.GetApp(app).UrlIsFDroidRepo {
		// Create separate variable to get around not being able to address map items
		err := dl.DownloadFromFDroidRepo(apps.GetApp(app), zip, ver, arch, apppath)
		if err != nil {
			return err
		}
	} else {
		err := dl.Download(apps.GetAppVersionArch(app, ver, arch).Url, apppath)
		if err != nil {
			return fmt.Errorf("Error while downloading %v to %v:\n  %v",
				apps.GetAppVersionArch(app, ver, arch).Url, apppath, err)
		}
	}
	return nil
}

func DownloadApp(zip *lib.ZipInfo, files *lib.Files, apps *lib.Apps, app, ver, arch, zippath string, ch chan error, wg *sync.WaitGroup) {
	defer wg.Done()
	apps.RLockAppVersion(app, ver)
	defer apps.RUnlockAppVersion(app, ver)
	if apps.AppVersionArchExists(app, ver, arch) {
		apps.LockAppVersionArch(app, ver, arch)
		defer apps.UnlockAppVersionArch(app, ver, arch)
		// Build file name with version/architecture
		filename := apps.GetApp(app).PackageName + "-" + apps.GetAppVersion(app, ver).Base
		if apps.GetAppVersion(app, ver).HasArchSpecificInfo {
			filename = filename + "-" + arch
		}
		filename = filename + ".apk"
		lib.Debug("APP " + app + " FOR " + ver + " " + arch + " WILL BE SAVED AS " + filename)
		// Set file name for app
		apps.GetAppVersionArch(app, ver, arch).FileName = filename
		apppath := filepath.Join(zippath, "files", filename)

		// Download as necessary
		err := downloadApp(apps, zip, app, ver, arch, apppath)
		if err != nil {
			ch <- fmt.Errorf("Error while downloading app \"%v\":\n  %v", apps.GetApp(app).PackageName, err)
			return
		}

		// Test checksums
		err = checkChecksums(apps.GetAppVersionArch(app, ver, arch), apppath)
		if err != nil {
			ch <- err
			return
		}

		err = unzipSystemLibs(zippath, zip, apps.GetApp(app), ver, arch, files)
		if err != nil {
			ch <- fmt.Errorf("Error while unzipping libs from %v:\n  %v", apps.GetApp(app).PackageName, err)
			return
		}
	}
}
