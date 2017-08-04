package build

import (
    "archive/zip"
	"dl"
    "io"
	"lib"
    "log"
	"os"
	"path/filepath"
    "strings"
	"github.com/spf13/viper"
)

func zipFolder(root string, zipinfo lib.ZipInfo) string {
    zipdest := filepath.Join(viper.GetString("destination"), zipinfo.Name + ".zip")
    log.Println("Creating zip file at " + zipdest)
    // Create destination directory if it doesn't exist
    os.MkdirAll(viper.GetString("destination"), os.ModeDir | 0755)
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
func MakeZip(zip lib.ZipInfo, apps map[string]lib.AppInfo, files map[string]lib.FileInfo) {
	zippath := filepath.Join(viper.GetString("tempdir"), "build", zip.Name)
    // Build zip root with files subdir
    err := os.MkdirAll(filepath.Join(zippath, "files"), os.ModeDir | 0755)
	//defer os.RemoveAll(zippath)
	lib.ExitIfError(err)

	// Download apps
	for _, app := range zip.Apps {
		if apps[app].FileInfo.Url != "" {
			apppath := filepath.Join(zippath, "files", apps[app].PackageName+".apk")
			if apps[app].UrlIsFDroidRepo {
                // Create separate variable to get around not being able to address map items
                appRef := apps[app]
                log.Println("Downloading " + appRef.FileInfo.FileName + " from F-Droid repo at " + appRef.FileInfo.Url)
				dl.DownloadFromFDroidRepo(&appRef, apppath)
                apps[app] = appRef
			} else {
                log.Println("Downloading " + apps[app].FileInfo.FileName)
				dl.Download(apps[app].FileInfo.Url, apppath)
			}
			// TODO: Verify hash of file, error on mismatch
		}
	}
	// Download other files
	for _, file := range zip.Files {
		if files[file].Url != "" {
			filepath := filepath.Join(zippath, "files", files[file].FileName)
            log.Println("Downloading " + files[file].FileName)
			dl.Download(files[file].Url, filepath)
			// TODO: Verify hash of file, error on mismatch
		}
	}
	
	makePermsFile(zippath, &zip, apps, &files)
    makeSysconfigFile(zippath, &zip, apps, &files)
	makeAddondScript(zippath, &zip, apps, &files)
	makeUpdaterScript(zippath, zip, apps, files)
    // Source is hardcoded because I know it will not change until I change it
    log.Println("Downloading update-binary")
    dl.Download("https://gitlab.com/Shadow53/zip-builder/raw/master/update-binary", filepath.Join(zippath, "META-INF", "com", "google", "android", "update-binary"))
    
    // Generate zip and md5 file
    lib.GenerateMD5File(zipFolder(zippath, zip))
}
