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

	out, err := ioutil.TempFile(viper.GetString("tempdir"), "zip-builder-")
	if err != nil {
		return fmt.Errorf("Error while creating a file at %v:\n  %v", dest, err)
	}
	defer out.Close()

	resp, err := http.Get(src)
	if err != nil {
		return fmt.Errorf("Error while setting up a connection to %v:\n  %v", src, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Error while connecting to %v:\n  Received non-ok status code %v", src, resp.StatusCode)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("Error while writing to the file at %v:\n  %v", dest, err)
	}

	err = os.Rename(out.Name(), dest)
	if err != nil {
		return fmt.Errorf("Error while moving temporary file from %v to %v:\n  %v", out.Name(), dest, err)
	}

	return nil
}

func getFDroidRepoIndex(urlstr string) (string, error) {
	url, err := url.Parse(urlstr)
	if err != nil {
		return "", fmt.Errorf("Error while parsing %v as a URL:\n  %v", urlstr, err)
	}
	dest := filepath.Join(viper.GetString("tempdir"), url.Host+".xml")
	return dest, Download(urlstr+"/index.xml", dest)
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

func DownloadFromFDroidRepo(app *lib.AppInfo, zip *lib.ZipInfo, ver, arch, dest string) error {
	lib.Debug("DOWNLOADING " + app.PackageName + " FROM F-DROID")
	if app.Android.Version[ver].Arch[arch].Url != "" {
		index, err := getFDroidRepoIndex(app.Android.Version[ver].Arch[arch].Url)
		if err != nil {
			return fmt.Errorf("Error while downloading %v from %v:\n  %v", app.PackageName,
				app.Android.Version[ver].Arch[arch].Url, err)
		}
		// Read contents of index file and parse XML for desired info
		bytes, err := ioutil.ReadFile(index)
		if err != nil {
			return fmt.Errorf("Error while reading F-Droid index from %v:\n  %v",
				app.Android.Version[ver].Arch[arch].Url, err)
		}

		var repoInfo FDroidRepo
		err = xml.Unmarshal(bytes, &repoInfo)
		if err != nil {
			return fmt.Errorf("Error while parsing XML from %v:\n  %v",
				app.Android.Version[ver].Arch[arch].Url, err)
		}

		// Navigate parsed XML for information on the desired package
		for _, tmpapp := range repoInfo.Apps {
			if tmpapp.Id == app.PackageName {
				lib.Debug("ADDING PERMISSIONS LISTED ON F-DROID")
				app.Permissions = strings.Split(tmpapp.Apks[0].Permissions, ",")
				for _, ver := range zip.Versions {
					if app.Android.Version[ver] != nil && app.Android.Version[ver].Base != "" {
						if app.Android.Version[ver].HasArchSpecificInfo {
							for _, arch := range zip.Arches {
								file := app.Android.Version[ver].Arch[arch]
								switch tmpapp.Apks[0].Hash.Type {
								case "md5":
									file.MD5 = tmpapp.Apks[0].Hash.Hash
								case "sha1":
									file.SHA1 = tmpapp.Apks[0].Hash.Hash
								case "sha256":
									file.SHA256 = tmpapp.Apks[0].Hash.Hash
								}
								app.Android.Version[ver].Arch[arch] = file
							}
						} else {
							file := app.Android.Version[ver].Arch[lib.NOARCH]
							switch tmpapp.Apks[0].Hash.Type {
							case "md5":
								file.MD5 = tmpapp.Apks[0].Hash.Hash
							case "sha1":
								file.SHA1 = tmpapp.Apks[0].Hash.Hash
							case "sha256":
								file.SHA256 = tmpapp.Apks[0].Hash.Hash
							}
							app.Android.Version[ver].Arch[lib.NOARCH] = file
						}
					}
				}
				// Download file and store file locations
				err := Download(app.Android.Version[ver].Arch[arch].Url+"/"+tmpapp.Apks[0].FileName, dest)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
