package lib

import (
	"bytes"
	"sync"
)

type Files struct {
	File map[string]*AndroidVersions
	Mux  sync.RWMutex
}

func (f *Files) String() string {
	var buf bytes.Buffer
	buf.WriteString("Files{")
	for key, val := range f.File {
		buf.WriteString("\n  ")
		buf.WriteString(key)
		buf.WriteString(": ")
		buf.WriteString(val.String())
	}
	buf.WriteString("\n}")
	return buf.String()
}

func (f *Files) Lock() {
	f.Mux.Lock()
}

func (f *Files) RLock() {
	f.Mux.RLock()
}

func (f *Files) Unlock() {
	f.Mux.Unlock()
}

func (f *Files) RUnlock() {
	f.Mux.RUnlock()
}

func (f *Files) FileExists(name string) bool {
	return f.File[name] != nil
}

func (f *Files) GetFile(name string) *AndroidVersions {
	return f.File[name]
}

func (f *Files) SetFile(name string, file *AndroidVersions) {
	f.File[name] = file
}

func (f *Files) LockFile(name string) {
	f.RLock()
	f.File[name].Mux.Lock()
}

func (f *Files) RLockFile(name string) {
	f.RLock()
	f.File[name].Mux.RLock()
}

func (f *Files) UnlockFile(name string) {
	f.RUnlock()
	f.File[name].Mux.Unlock()
}

func (f *Files) RUnlockFile(name string) {
	f.RUnlock()
	f.File[name].Mux.RUnlock()
}

func (f *Files) FileVersionExists(name, ver string) bool {
	return f.GetFileVersion(name, ver) != nil
}

func (f *Files) GetFileVersion(name, ver string) *AndroidVersionInfo {
	if !f.FileExists(name) {
		return nil
	}
	return f.File[name].Version[ver]
}

func (f *Files) SetFileVersion(name, ver string, file *AndroidVersionInfo) {
	f.File[name].Version[ver] = file
}

func (f *Files) LockFileVersion(name, ver string) {
	f.RLockFile(name)
	f.File[name].Version[ver].Mux.Lock()
}

func (f *Files) RLockFileVersion(name, ver string) {
	f.RLockFile(name)
	f.File[name].Version[ver].Mux.RLock()
}

func (f *Files) UnlockFileVersion(name, ver string) {
	f.RUnlockFile(name)
	f.File[name].Version[ver].Mux.Unlock()
}

func (f *Files) RUnlockFileVersion(name, ver string) {
	f.RUnlockFile(name)
	f.File[name].Version[ver].Mux.RUnlock()
}

func (f *Files) FileVersionArchExists(name, ver, arch string) bool {
	return f.GetFileVersionArch(name, ver, arch) != nil
}

func (f *Files) GetFileVersionArch(name, ver, arch string) *FileInfo {
	if !f.FileVersionExists(name, ver) {
		return nil
	}
	return f.File[name].Version[ver].Arch[arch]
}

func (f *Files) SetFileVersionArch(name, ver, arch string, file *FileInfo) {
	f.File[name].Version[ver].Arch[arch] = file
}

func (f *Files) LockFileVersionArch(name, ver, arch string) {
	f.RLockFileVersion(name, ver)
	f.File[name].Version[ver].Arch[arch].Mux.Lock()
}

func (f *Files) RLockFileVersionArch(name, ver, arch string) {
	f.RLockFileVersion(name, ver)
	f.File[name].Version[ver].Arch[arch].Mux.RLock()
}

func (f *Files) UnlockFileVersionArch(name, ver, arch string) {
	f.RUnlockFileVersion(name, ver)
	f.File[name].Version[ver].Arch[arch].Mux.Unlock()
}

func (f *Files) RUnlockFileVersionArch(name, ver, arch string) {
	f.RUnlockFileVersion(name, ver)
	f.File[name].Version[ver].Arch[arch].Mux.RUnlock()
}
