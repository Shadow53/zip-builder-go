package lib

import (
	"bytes"
	"sync"
)

type Apps struct {
	App map[string]*AppInfo
	Mux sync.RWMutex
}

func (a *Apps) String() string {
	var buf bytes.Buffer
	buf.WriteString("Apps{\n")
	for key, app := range a.App {
		buf.WriteString("  ")
		buf.WriteString(key)
		buf.WriteString(": ")
		buf.WriteString(app.String())
		buf.WriteString("\n")
	}
	buf.WriteString("}")
	return buf.String()
}

func (a *Apps) Lock() {
	a.Mux.Lock()
}

func (a *Apps) RLock() {
	a.Mux.RLock()
}

func (a *Apps) Unlock() {
	a.Mux.Unlock()
}

func (a *Apps) RUnlock() {
	a.Mux.RUnlock()
}

func (a *Apps) AppExists(name string) bool {
	return a.App[name] != nil
}

func (a *Apps) GetApp(name string) *AppInfo {
	return a.App[name]
}

func (a *Apps) SetApp(name string, app *AppInfo) {
	a.App[name] = app
}

func (a *Apps) LockApp(name string) {
	a.RLock()
	a.App[name].Mux.Lock()
}

func (a *Apps) RLockApp(name string) {
	a.RLock()
	a.App[name].Mux.RLock()
}

func (a *Apps) UnlockApp(name string) {
	a.RUnlock()
	a.App[name].Mux.Unlock()
}

func (a *Apps) RUnlockApp(name string) {
	a.RUnlock()
	a.App[name].Mux.RUnlock()
}

func (a *Apps) AppVersionExists(name, ver string) bool {
	return a.GetAppVersion(name, ver) != nil
}

func (a *Apps) GetAppVersion(name, ver string) *AndroidVersionInfo {
	if !a.AppExists(name) {
		return nil
	}
	return a.App[name].Android.Version[ver]
}

func (a *Apps) SetAppVersion(name, ver string, version *AndroidVersionInfo) {
	a.App[name].Android.Version[ver] = version
}

func (a *Apps) LockAppVersion(name, ver string) {
	a.RLockApp(name)
	a.App[name].Android.Version[ver].Mux.Lock()
}

func (a *Apps) RLockAppVersion(name, ver string) {
	a.RLockApp(name)
	a.App[name].Android.Version[ver].Mux.RLock()
}

func (a *Apps) UnlockAppVersion(name, ver string) {
	a.RUnlockApp(name)
	a.App[name].Android.Version[ver].Mux.Unlock()
}

func (a *Apps) RUnlockAppVersion(name, ver string) {
	a.RUnlockApp(name)
	a.App[name].Android.Version[ver].Mux.RUnlock()
}

func (a *Apps) AppVersionArchExists(name, ver, arch string) bool {
	return a.GetAppVersionArch(name, ver, arch) != nil
}

func (a *Apps) GetAppVersionArch(name, ver, arch string) *FileInfo {
	if !a.AppVersionExists(name, ver) {
		return nil
	}
	return a.App[name].Android.Version[ver].Arch[arch]
}

func (a *Apps) SetAppVersionArch(name, ver, arch string, archInfo *FileInfo) {
	a.App[name].Android.Version[ver].Arch[arch] = archInfo
}

func (a *Apps) LockAppVersionArch(name, ver, arch string) {
	a.RLockAppVersion(name, ver)
	a.App[name].Android.Version[ver].Arch[arch].Mux.Lock()
}

func (a *Apps) RLockAppVersionArch(name, ver, arch string) {
	a.RLockAppVersion(name, ver)
	a.App[name].Android.Version[ver].Arch[arch].Mux.RLock()
}

func (a *Apps) UnlockAppVersionArch(name, ver, arch string) {
	a.RUnlockAppVersion(name, ver)
	a.App[name].Android.Version[ver].Arch[arch].Mux.Unlock()
}

func (a *Apps) RUnlockAppVersionArch(name, ver, arch string) {
	a.RUnlockAppVersion(name, ver)
	a.App[name].Android.Version[ver].Arch[arch].Mux.RUnlock()
}
