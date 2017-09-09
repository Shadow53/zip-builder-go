package lib

import "sync"

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

type AndroidVersionInfo struct {
	HasArchSpecificInfo bool   // Architectures were set in config. If false, just read from Arm
	Base                string // Which Android version's config this was based on
	Arch                map[string]*FileInfo
	Mux                 sync.RWMutex
}

type AndroidVersions struct {
	Version map[string]*AndroidVersionInfo
	Mux     sync.RWMutex
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
