package lib

import (
	"bytes"
	"fmt"
	"sync"
)

type FileInfo struct {
	Url                string
	Destination        string
	InstallRemoveFiles []string
	UpdateRemoveFiles  []string
	Hash               string
	Mode               string
	FileName           string
	MD5                string
	SHA1               string
	SHA256             string
	Mux                sync.RWMutex
}

func (f *FileInfo) String() string {
	var buf bytes.Buffer
	buf.WriteString("FileInfo{\n  URL: ")
	buf.WriteString(f.Url)
	buf.WriteString("\n  Destination: ")
	buf.WriteString(f.Destination)
	buf.WriteString("\n  InstallRemoveFiles: ")
	buf.WriteString(fmt.Sprintf("%v", f.InstallRemoveFiles))
	buf.WriteString("\n  UpdateRemoveFiles: ")
	buf.WriteString(fmt.Sprintf("%v", f.UpdateRemoveFiles))
	buf.WriteString("\n  Hash: ")
	buf.WriteString(f.Hash)
	buf.WriteString("\n  Mode: ")
	buf.WriteString(f.Mode)
	buf.WriteString("\n  FileName: ")
	buf.WriteString(f.FileName)
	buf.WriteString("\n  MD5: ")
	buf.WriteString(f.MD5)
	buf.WriteString("\n  SHA1: ")
	buf.WriteString(f.SHA1)
	buf.WriteString("\n  SHA256: ")
	buf.WriteString(f.SHA256)
	buf.WriteString("\n}")
	return buf.String()
}

type AndroidVersionInfo struct {
	HasArchSpecificInfo bool   // Architectures were set in config. If false, just read from Arm
	Base                string // Which Android version's config this was based on
	Arch                map[string]*FileInfo
	Mux                 sync.RWMutex
}

func (av *AndroidVersionInfo) String() string {
	var buf bytes.Buffer
	buf.WriteString("AndroidVersionInfo{\n  HasArchSpecificInfo: ")
	buf.WriteString(fmt.Sprintf("%v", av.HasArchSpecificInfo))
	buf.WriteString("\n  Base: ")
	buf.WriteString(av.Base)
	buf.WriteString("\n  Arch: {")
	for key, val := range av.Arch {
		buf.WriteString("\n    ")
		buf.WriteString(key)
		buf.WriteString(": ")
		buf.WriteString(val.String())
	}
	buf.WriteString("\n  }\n}")
	return buf.String()
}

type AndroidVersions struct {
	Version map[string]*AndroidVersionInfo
	Mux     sync.RWMutex
}

func (av *AndroidVersions) String() string {
	var buf bytes.Buffer
	buf.WriteString("AndroidVersions{")
	for key, val := range av.Version {
		buf.WriteString("\n  ")
		buf.WriteString(key)
		buf.WriteString(": ")
		buf.WriteString(val.String())
	}
	buf.WriteString("\n}")
	return buf.String()
}

type AppInfo struct {
	PackageName             string
	UrlIsFDroidRepo         bool
	DozeWhitelist           bool
	DozeWhitelistExceptIdle bool
	DataSaverWhitelist      bool
	AllowSystemUser         bool
	BlacklistSystemUser     bool
	Android                 AndroidVersions
	Permissions             []string
	Mux                     sync.RWMutex
}

func (a *AppInfo) String() string {
	var buf bytes.Buffer
	buf.WriteString("AppInfo{")
	buf.WriteString("\n  PackageName: ")
	buf.WriteString(a.PackageName)
	buf.WriteString("\n  UrlIsFDroidRepo: ")
	buf.WriteString(fmt.Sprintf("%v", a.UrlIsFDroidRepo))
	buf.WriteString("\n  DozeWhitelist: ")
	buf.WriteString(fmt.Sprintf("%v", a.DozeWhitelist))
	buf.WriteString("\n  DozeWhitelistExceptIdle: ")
	buf.WriteString(fmt.Sprintf("%v", a.DozeWhitelistExceptIdle))
	buf.WriteString("\n  AllowSystemUser: ")
	buf.WriteString(fmt.Sprintf("%v", a.AllowSystemUser))
	buf.WriteString("\n  BlacklistSystemUser: ")
	buf.WriteString(fmt.Sprintf("%v", a.BlacklistSystemUser))
	buf.WriteString("\n  Android: ")
	buf.WriteString(a.Android.String())
	buf.WriteString("\n  Permissions: ")
	buf.WriteString(fmt.Sprintf("%v", a.Permissions))
	buf.WriteString("\n}")
	return buf.String()
}

type ZipInfo struct {
	Name               string
	Arch               string
	SdkVersion         string
	InstallRemoveFiles []string
	UpdateRemoveFiles  []string
	Apps               []string
	Files              []string
	Arches             []string
	Versions           []string
	Mux                sync.RWMutex
}

func (z *ZipInfo) String() string {
	var buf bytes.Buffer
	buf.WriteString("ZipInfo{")
	buf.WriteString("\n  Name: ")
	buf.WriteString(z.Name)
	buf.WriteString("\n  Arch: ")
	buf.WriteString(z.Arch)
	buf.WriteString("\n  SdkVersion: ")
	buf.WriteString(z.SdkVersion)
	buf.WriteString("\n  InstallRemoveFiles: ")
	buf.WriteString(fmt.Sprintf("%v", z.InstallRemoveFiles))
	buf.WriteString("\n  UpdateRemoveFiles: ")
	buf.WriteString(fmt.Sprintf("%v", z.UpdateRemoveFiles))
	buf.WriteString("\n  Apps: ")
	buf.WriteString(fmt.Sprintf("%v", z.Apps))
	buf.WriteString("\n  Files: ")
	buf.WriteString(fmt.Sprintf("%v", z.Files))
	buf.WriteString("\n  Arches: ")
	buf.WriteString(fmt.Sprintf("%v", z.Arches))
	buf.WriteString("\n  Versions: ")
	buf.WriteString(fmt.Sprintf("%v", z.Versions))
	buf.WriteString("\n}")
	return buf.String()
}

func (z *ZipInfo) Lock() {
	z.Mux.Lock()
}

func (z *ZipInfo) RLock() {
	z.Mux.RLock()
}

func (z *ZipInfo) Unlock() {
	z.Mux.Unlock()
}

func (z *ZipInfo) RUnlock() {
	z.Mux.RUnlock()
}
