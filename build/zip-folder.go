package build

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gitlab.com/Shadow53/zip-builder/lib"
)

func zipFolder(root string, zipinfo *lib.ZipInfo) (string, error) {
	zipinfo.RLock()
	zipdest := filepath.Join(viper.GetString("destination"), zipinfo.Name+".zip")
	zipinfo.RUnlock()

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
