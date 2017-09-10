package build

import (
	"fmt"
	"path/filepath"
	"sync"

	"gitlab.com/Shadow53/zip-builder/dl"
	"gitlab.com/Shadow53/zip-builder/lib"
)

func DownloadFile(files *lib.Files, file, ver, arch, zippath string, cherr chan error, wg *sync.WaitGroup) {
	defer wg.Done()
	files.RLockFileVersion(file, ver)
	defer files.RUnlockFileVersion(file, ver)
	if files.FileVersionArchExists(file, ver, arch) {
		files.LockFileVersionArch(file, ver, arch)
		defer files.UnlockFileVersionArch(file, ver, arch)
		if files.GetFileVersionArch(file, ver, arch).Url != "" {
			filename := files.GetFileVersionArch(file, ver, arch).FileName + "." + ver
			if files.GetFileVersion(file, ver).HasArchSpecificInfo {
				filename = filename + "." + arch
			}
			lib.Debug("FILE " + file + " FOR " + ver + " " + arch + " WILL BE SAVED AS " + filename)
			files.GetFileVersionArch(file, ver, arch).FileName = filename
			filepath := filepath.Join(zippath, "files", filename)

			err := dl.Download(files.GetFileVersionArch(file, ver, arch).Url, filepath)
			if err != nil {
				cherr <- fmt.Errorf("Error while downloading %v:\n  %v", files.File[file].Version[ver].Arch[arch].Url, err)
				return
			}
			// Test checksums
			err = checkChecksums(files.GetFileVersionArch(file, ver, arch), filepath)
			if err != nil {
				cherr <- fmt.Errorf("Error while downloading file \"%v\":\n %v", file, err)
				return
			}
		} else {
			lib.Debug("WARNING: URL IS EMPTY FOR " + file)
		}
	}
}
