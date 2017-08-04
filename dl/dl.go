package dl

import (
	"encoding/xml"
	"io"
	"io/ioutil"
	"lib"
    "log"
	"net/http"
	"net/url"
	"os"
    "path/filepath"
	"strings"

	"github.com/spf13/viper"
)

func Download(src, dest string) {
    log.Println("Downloading " + src)
	out, err := os.Create(dest)
	defer out.Close()
	lib.ExitIfError(err)

	resp, err := http.Get(src)
	defer resp.Body.Close()
	lib.ExitIfError(err)

	_, err = io.Copy(out, resp.Body)
	lib.ExitIfError(err)
}

func downloadToTempDir(urlstr, filename string, useExisting bool) string {
	dest := filepath.Join(viper.GetString("tempdir"), filename)
	if _, err := os.Stat("dest"); !useExisting || err != nil {
		if !useExisting || os.IsNotExist(err) {
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

func DownloadFromFDroidRepo(app *lib.AppInfo, dest string) {
	// Get path to repo index file
	index := getFDroidRepoIndex(app.FileInfo.Url)

	// Read contents of index file and parse XML for desired info
	bytes, err := ioutil.ReadFile(index)
	lib.ExitIfError(err)
	var repoInfo FDroidRepo
	err = xml.Unmarshal(bytes, &repoInfo)
	lib.ExitIfError(err)

	// Navigate parsed XML for information on the desired package
	for _, tmpapp := range repoInfo.Apps {
		if tmpapp.Id == app.PackageName {
			app.Permissions = strings.Split(tmpapp.Apks[0].Permissions, ",")
			// Download file and store file locations
			tmppath := downloadToTempDir(app.FileInfo.Url+"/"+tmpapp.Apks[0].FileName, app.PackageName+".apk", false)
			err = os.Rename(tmppath, dest)
			lib.ExitIfError(err)
		}
	}
}
