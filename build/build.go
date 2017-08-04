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

// Only extracts libraries if being installed under /system
// To be fair, that's almost always where apps get installed...
func unzipSystemLibs(root string, zipinfo *lib.ZipInfo, app lib.AppInfo, files *map[string]lib.FileInfo) {
    if strings.HasPrefix(app.FileInfo.Destination, "/system/") {
        // Hold all library files for this app in {ZIPROOT}/files/app-lib/
        destFolder := filepath.Join(root, "files", app.PackageName + "-lib")
        
        reader, err := zip.OpenReader(filepath.Join(root, "files", app.FileInfo.FileName))
        lib.ExitIfError(err)
        
        // Extract only files whose paths begin with "lib/"
        for _, file := range reader.File {
            if strings.HasPrefix(file.Name, "lib/") {
                log.Println("Extracting libraries from " + app.PackageName)
                // Only create the parent folder if there is a file to extract
                err = os.MkdirAll(destFolder, os.ModeDir | 0755)
                lib.ExitIfError(err)
                
                fileName := file.Name[strings.LastIndex(file.Name, "lib/")+4:]
                path := filepath.Join(destFolder, fileName)
                os.MkdirAll(path[:strings.LastIndex(path, "/")], os.ModeDir | 0755)
                
                fileReader, err := file.Open()
                lib.ExitIfError(err)
                defer fileReader.Close()
                
                targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
                lib.ExitIfError(err)
                defer targetFile.Close()
                
                _, err = io.Copy(targetFile, fileReader)
                lib.ExitIfError(err)
                
                // Will only be reached by actual files
                fileId := app.PackageName + "-" + file.Name
                // If file extracted correctly without problems, add to list
                (*files)[fileId] = lib.FileInfo{
                    Destination: app.FileInfo.Destination[:strings.LastIndex(app.FileInfo.Destination, "/")] + "/" + file.Name,
                    Mode:        "0644",
                    FileName:    app.PackageName + "-lib/" + fileName }
                    
                zipinfo.Files = append(zipinfo.Files, fileId)
            }
        }
    }
}

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
                dl.DownloadFromFDroidRepo(&appRef, apppath)
                apps[app] = appRef
			} else {
                dl.Download(apps[app].FileInfo.Url, apppath)
			}
			unzipSystemLibs(zippath, &zip, apps[app], &files)
			// TODO: Verify hash of file, error on mismatch
		}
	}
	// Download other files
	for _, file := range zip.Files {
		if files[file].Url != "" {
			filepath := filepath.Join(zippath, "files", files[file].FileName)
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
