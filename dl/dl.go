package dl

import (
	"encoding/xml"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gitlab.com/Shadow53/zip-builder/lib"
)

func Download(src, dest string) {
	log.Println("Downloading " + src)
	lib.Debug("SOURCE URL: " + src)
	lib.Debug("DESTINATION: " + dest)

	out, err := os.Create(dest)
	lib.ExitIfError(err)
	defer out.Close()

	resp, err := http.Get(src)
	lib.ExitIfError(err)
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	lib.ExitIfError(err)
}

func downloadToTempDir(urlstr, filename string, useExisting bool) string {
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
			lib.ExitIfError(err)
		}
	}
	return dest
}

func getFDroidRepoIndex(urlstr string) string {
	url, err := url.Parse(urlstr)
	lib.ExitIfError(err)
	return downloadToTempDir(urlstr+"/index.xml", url.Host+".xml", true)
}

type FDroidHash struct {
	Type string `xml:",attr"`
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

func DownloadFromFDroidRepo(app *lib.AppInfo, ver, arch, dest string) {
	lib.Debug("DOWNLOADING " + app.PackageName + " FROM F-DROID")
	if app.AndroidVersion[ver].Arch[arch].Url != "" {
		index := getFDroidRepoIndex(app.AndroidVersion[ver].Arch[arch].Url)

		// Read contents of index file and parse XML for desired info
		bytes, err := ioutil.ReadFile(index)
		lib.ExitIfError(err)
		var repoInfo FDroidRepo
		err = xml.Unmarshal(bytes, &repoInfo)
		lib.ExitIfError(err)

		// Navigate parsed XML for information on the desired package
		for _, tmpapp := range repoInfo.Apps {
			if tmpapp.Id == app.PackageName {
				lib.Debug("ADDING PERMISSIONS LISTED ON F-DROID")
				app.Permissions = strings.Split(tmpapp.Apks[0].Permissions, ",")
				// Download file and store file locations
				tmppath := downloadToTempDir(app.AndroidVersion[ver].Arch[arch].Url+"/"+tmpapp.Apks[0].FileName, app.PackageName+".apk", false)
				err = os.Rename(tmppath, dest)
				lib.ExitIfError(err)
			}
		}
	}
}
