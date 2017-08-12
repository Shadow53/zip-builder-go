package dl

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gitlab.com/Shadow53/zip-builder/lib"
)

func Download(src, dest string) error {
	fmt.Println("Downloading " + src)
	lib.Debug("SOURCE URL: " + src)
	lib.Debug("DESTINATION: " + dest)

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("Error while creating a file at %v:\n  %v", dest, err)
	}
	defer out.Close()

	resp, err := http.Get(src)
	if err != nil {
		return fmt.Errorf("Error while setting up a connection to %v:\n  %v", src, err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("Error while writing to the file at %v:\n  %v", dest, err)
	}

	return nil
}

func downloadToTempDir(urlstr, filename string, useExisting bool) (string, error) {
	dest := filepath.Join(viper.GetString("tempdir"), filename)
	if _, err := os.Stat("dest"); !useExisting || err != nil {
		if !useExisting || os.IsNotExist(err) {
			if os.IsNotExist(err) {
				lib.Debug("FILE " + filename + " DOES NOT EXIST")
			} else {
				lib.Debug("IGNORING FILE IF EXISTS")
			}
			Download(urlstr, dest)
		} else {
			return "", fmt.Errorf("Error while testing file at %v:\n  %v", filename, err)
		}
	}
	return dest, nil
}

func getFDroidRepoIndex(urlstr string) (string, error) {
	url, err := url.Parse(urlstr)
	if err != nil {
		return "", fmt.Errorf("Error while parsing %v as a URL:\n  %v", urlstr, err)
	}
	return downloadToTempDir(urlstr+"/index.xml", url.Host+".xml", true)
}

type FDroidHash struct {
	Type string `xml:"type,attr"`
	Hash string `xml:",chardata"`
}

type FDroidApk struct {
	Version     string     `xml:"version"`
	FileName    string     `xml:"apkname"`
	Hash        FDroidHash `xml:"hash"`
	Permissions string     `xml:"permissions"`
}

type FDroidApp struct {
	Id   string      `xml:"id,attr"`
	Name string      `xml:"name"`
	Apks []FDroidApk `xml:"package"`
}

type FDroidRepo struct {
	XMLName xml.Name    `xml:"fdroid"`
	Apps    []FDroidApp `xml:"application"`
}

func DownloadFromFDroidRepo(app *lib.AppInfo, ver, arch, dest string) error {
	lib.Debug("DOWNLOADING " + app.PackageName + " FROM F-DROID")
	if app.AndroidVersion[ver].Arch[arch].Url != "" {
		index, err := getFDroidRepoIndex(app.AndroidVersion[ver].Arch[arch].Url)
		if err != nil {
			return fmt.Errorf("Error while downloading %v from %v:\n  %v", app.PackageName,
				app.AndroidVersion[ver].Arch[arch].Url, err)
		}
		// Read contents of index file and parse XML for desired info
		bytes, err := ioutil.ReadFile(index)
		if err != nil {
			return fmt.Errorf("Error while reading F-Droid index from %v:\n  %v",
				app.AndroidVersion[ver].Arch[arch].Url, err)
		}

		var repoInfo FDroidRepo
		err = xml.Unmarshal(bytes, &repoInfo)
		if err != nil {
			return fmt.Errorf("Error while parsing XML from %v:\n  %v",
				app.AndroidVersion[ver].Arch[arch].Url, err)
		}

		// Navigate parsed XML for information on the desired package
		for _, tmpapp := range repoInfo.Apps {
			if tmpapp.Id == app.PackageName {
				lib.Debug("ADDING PERMISSIONS LISTED ON F-DROID")
				app.Permissions = strings.Split(tmpapp.Apks[0].Permissions, ",")
				for _, ver := range lib.Versions {
					if app.AndroidVersion[ver].Base != "" {
						if app.AndroidVersion[ver].HasArchSpecificInfo {
							for _, arch := range lib.Arches {
								file := app.AndroidVersion[ver].Arch[arch]
								switch tmpapp.Apks[0].Hash.Type {
								case "md5":
									file.MD5 = tmpapp.Apks[0].Hash.Hash
								case "sha1":
									file.SHA1 = tmpapp.Apks[0].Hash.Hash
								case "sha256":
									file.SHA256 = tmpapp.Apks[0].Hash.Hash
								}
								app.AndroidVersion[ver].Arch[arch] = file
							}
						} else {
							file := app.AndroidVersion[ver].Arch[lib.Arches[0]]
							switch tmpapp.Apks[0].Hash.Type {
							case "md5":
								file.MD5 = tmpapp.Apks[0].Hash.Hash
							case "sha1":
								file.SHA1 = tmpapp.Apks[0].Hash.Hash
							case "sha256":
								file.SHA256 = tmpapp.Apks[0].Hash.Hash
							}
							app.AndroidVersion[ver].Arch[lib.Arches[0]] = file
						}
					}
				}
				// Download file and store file locations
				tmppath, err := downloadToTempDir(app.AndroidVersion[ver].Arch[arch].Url+"/"+tmpapp.Apks[0].FileName, app.PackageName+".apk", false)
				if err != nil {
					return err
				}
				err = os.Rename(tmppath, dest)
				if err != nil {
					return fmt.Errorf("Error while moving temporary file from %v to %v:\n  %v", tmppath, dest, err)
				}
			}
		}
	}
	return nil
}
