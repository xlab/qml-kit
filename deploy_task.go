// v0 // THIS FILE MAY BE OVERWRITTEN BY UPDATE
// deploy_task.go — Go/QML deployment routines, part of the go-qml-kit.
//
// Authors:
//     Maxim Kouprianov <max@kc.vc>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the “Software”), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// +build gotask

package main

import (
	"bytes"
	"debug/macho"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/jingweno/gotask/tasking"
	"gopkg.in/yaml.v1"
)

const (
	outDir           = "out"
	relinkBase       = "@executable_path/../Frameworks"
	deployProfileSrc = "deploy_profile.yaml"
	wizardManifest   = "wizard.xml"
	wizardIcon       = "wizard_icon.png"
	docFile          = "doc.go"
)

var (
	verbose   = false
	deployers = map[string]func(*config, *tasking.T) error{
		"darwin":  deployDarwin,
		"windows": deployWindows,
		"linux":   deployLinux,
	}
)

type config struct {
	PkgInfo pkgInfo
	QtInfo  qtInfo
	Profile deployProfile
	Path    string
}

type deployProfile struct {
	Libs         map[string][]string
	Platforms    map[string][]string
	Modules      map[string][]string
	Imageformats []string
	Extra        map[string][]string
}

type qtInfo struct {
	Version, LibPath, BasePath string
}

type bundleInfo struct {
	Icon, Exec, Id string
}

type pkgInfo struct {
	Name, ImportPath string
}

// NAME
//	deploy - Run platform-specific deployment routine
//
// DESCRIPTION
// 	Embeds resources, compiles binary, copies related libs and plugins, etc...
//  Distribution-ready application package will be the result of this task.
//	Supported platforms are: darwin, linux, windows.
//
// OPTIONS
//	--verbose, -v
//		Enable some logging
//	--dmg
//		Create an installable dmg (darwin only)
func TaskDeploy(t *tasking.T) {
	deploy, ok := deployers[runtime.GOOS]
	if !ok {
		t.Fatal("deploy: platform unsupported:", runtime.GOOS)
	}

	verbose = t.Flags.Bool("verbose")

	// prepare output path
	path := fmt.Sprintf("%s/%s", outDir, runtime.GOOS)
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	// gather info
	pkgInfo, err := getPkgInfo()
	if err != nil {
		t.Fatal(err)
	}
	qtInfo, err := getQtInfo()
	if err != nil {
		t.Fatal(err)
	}
	// read deploy profile
	buf, err := ioutil.ReadFile(deployProfileSrc)
	if err != nil {
		t.Fatal(err)
	}
	var profile deployProfile
	err = yaml.Unmarshal(buf, &profile)
	if err != nil {
		t.Fatalf("deploy: profile: %v\n", err)
	} else if verbose {
		t.Log("deploy: profile loaded")
	}
	cfg := config{
		PkgInfo: pkgInfo,
		QtInfo:  qtInfo,
		Path:    path,
		Profile: profile,
	}
	if t.Flags.Bool("verbose") {
		t.Log("deploy: package name:", pkgInfo.Name)
		t.Logf("deploy: qt base: %s (%s)\n", qtInfo.BasePath, qtInfo.Version)
	}
	// embed resources
	if verbose {
		t.Log("deploy: embedding resources")
	}
	if err := exec.Command("rice", "embed-go").Run(); err != nil {
		t.Fatalf("deploy: rice: %v", err)
	}
	// run deployment
	if err := deploy(&cfg, t); err != nil {
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
		t.Fatal(err)
	}
	// clean leftovers
	if err := exec.Command("rice", "clean").Run(); err != nil {
		t.Fatalf("clean: rice: %v", err)
	}
}

// NAME
//	clean - Clean deployment leftovers and wizard configs
//
// DESCRIPTION
// 	Runs `rice clean` to remove leftovers from resource embedding,
//  purges wizard configs if any left in project dir.
//
// OPTIONS
//	--all, -a
//		Remove platform specific output dir
func TaskClean(t *tasking.T) {
	for _, name := range []string{wizardManifest, wizardIcon, docFile} {
		if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
			t.Fatalf("clean: %v", err)
		}
	}
	if err := exec.Command("rice", "clean").Run(); err != nil {
		t.Fatalf("clean: rice: %v", err)
	}
	if t.Flags.Bool("all") {
		path := fmt.Sprintf("%s/%s", outDir, runtime.GOOS)
		if len(path) < 2 {
			t.Fatal("can't happen")
		}
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
	}
}

// deployDarwin is a routine for Darwin (OS X)
func deployDarwin(cfg *config, t *tasking.T) (err error) {
	logprefix := "deploy [darwin]:"
	qlib := cfg.QtInfo.LibPath

	if verbose {
		t.Log(logprefix, "initial structure")
	}
	// target.app/Contents
	path := filepath.Join(cfg.Path, cfg.PkgInfo.Name+".app", "Contents")
	if err = os.MkdirAll(path, 0755); err != nil {
		return
	}
	// target.app/MacOS
	if err = os.Mkdir(filepath.Join(path, "MacOS"), 0755); err != nil {
		return
	}
	// target.app/Frameworks
	if err = os.Mkdir(filepath.Join(path, "Frameworks"), 0755); err != nil {
		return
	}
	// target.app/Plugins
	if err = os.Mkdir(filepath.Join(path, "Plugins"), 0755); err != nil {
		return
	}
	// target.app/Resources
	if err = os.Mkdir(filepath.Join(path, "Resources"), 0755); err != nil {
		return
	}
	// target.app/Info.plist
	if err = writeInfoPlist(path, cfg.PkgInfo); err != nil {
		return
	}
	// target.app/PkgInfo
	err = ioutil.WriteFile(filepath.Join(path, "PkgInfo"), []byte("APPL????\n"), 0666)
	if err != nil {
		return
	}
	// target.app/Resources/qt.conf
	err = ioutil.WriteFile(filepath.Join(path, "Resources", "qt.conf"), []byte(qtConfDarwin), 0666)
	if err != nil {
		return
	}
	// target.app/Resources/empty.lproj
	err = ioutil.WriteFile(filepath.Join(path, "Resources", "empty.lproj"), nil, 0666)
	if err != nil {
		return
	}
	if verbose {
		t.Log(logprefix, "building executable")
	}
	// target.app/MacOS/target
	name := filepath.Join(path, "MacOS", cfg.PkgInfo.Name)
	cmd := exec.Command("go", "build", "-o", name, cfg.PkgInfo.ImportPath)
	if _, err = cmd.Output(); err != nil {
		return fmt.Errorf("go build: %v", err)
	}
	if err = darwinRelink(qlib, name, true); err != nil {
		return
	}

	copyFw := func(fw string) (err error) {
		name := fw + ".framework"
		srcDir := filepath.Join(qlib, name, "Versions", "5")
		dstDir := filepath.Join(path, "Frameworks", name, "Versions", "5")
		err = os.MkdirAll(dstDir, 0755)
		if err != nil {
			return
		}
		err = copyFile(filepath.Join(srcDir, fw), filepath.Join(dstDir, fw))
		if err != nil {
			return
		}
		err = darwinRelink(qlib, filepath.Join(dstDir, fw), false)
		return
	}

	if verbose {
		t.Log(logprefix, "copying frameworks")
	}
	// target.app/Frameworks
	for _, fw := range cfg.Profile.Libs["default"] {
		if err = copyFw(fw); err != nil {
			return
		}
	}
	for _, fw := range cfg.Profile.Libs["darwin"] {
		if err = copyFw(fw); err != nil {
			return
		}
	}

	copyPlug := func(dir, name string) (err error) {
		name = "libq" + name + ".dylib"
		orig := filepath.Join(cfg.QtInfo.BasePath, "plugins", dir, name)
		targ := filepath.Join(path, "Plugins", dir, name)
		if err = copyFile(orig, targ); err != nil {
			return
		}
		if err = darwinRelink(qlib, targ, false); err != nil {
			return
		}
		return
	}

	if verbose {
		t.Log(logprefix, "copying plugins")
	}
	// target.app/Plugins/platforms
	err = os.Mkdir(filepath.Join(path, "Plugins", "platforms"), 0755)
	if err != nil {
		return
	}
	for _, name := range cfg.Profile.Platforms["darwin"] {
		if err = copyPlug("platforms", name); err != nil {
			return
		}
	}
	// target.app/Plugins/imageformats
	err = os.Mkdir(filepath.Join(path, "Plugins", "imageformats"), 0755)
	if err != nil {
		return
	}
	for _, name := range cfg.Profile.Imageformats {
		if err = copyPlug("imageformats", name); err != nil {
			return
		}
	}

	copyMod := func(name string) (err error) {
		orig := filepath.Join(cfg.QtInfo.BasePath, name)
		targ := filepath.Join(path, "Resources", name)
		err = os.MkdirAll(filepath.Dir(targ), 0755)
		if err != nil {
			return
		}
		if err = copyTree(orig, targ); err != nil {
			return
		}

		// relink any *.dyld found in module
		filepath.Walk(targ, func(rpath string, info os.FileInfo, err error) error {
			if info.Mode().IsRegular() && filepath.Ext(rpath) == ".dylib" {
				if err = darwinRelink(qlib, rpath, false); err != nil {
					return err
				}
			}
			return nil
		})
		return
	}

	if verbose {
		t.Log(logprefix, "copying modules")
	}
	// target.app/Resources/qml
	err = copyTree(filepath.Join("project", "qml"), filepath.Join(path, "Resources", "qml"))
	if err != nil {
		return
	}
	for category, mods := range cfg.Profile.Modules {
		dir := filepath.Join(strings.Split(category, "/")...)
		for _, name := range mods {
			targ := filepath.Join(dir, name)
			if err = copyMod(targ); err != nil {
				return
			}
		}
	}

	// disk image
	if t.Flags.Bool("dmg") {
		if verbose {
			t.Log(logprefix, "creating disk image")
		}
		err = os.Symlink("/Applications", filepath.Join(cfg.Path, "Applications"))
		if err != nil {
			return err
		}
		cmd := exec.Command("hdiutil", "create", "-srcfolder",
			cfg.Path, filepath.Join(cfg.Path, cfg.PkgInfo.Name+".dmg"))
		if err = cmd.Run(); err != nil {
			return err
		}
	}
	return
}

// deployLinux is a routine for Linux
func deployLinux(cfg *config, t *tasking.T) (err error) {
	logprefix := "deploy [linux]:"

	if verbose {
		t.Log(logprefix, "building executable")
	}
	name := filepath.Join(cfg.Path, cfg.PkgInfo.Name)
	cmd := exec.Command("go", "build", "-o", name, cfg.PkgInfo.ImportPath)
	if _, err = cmd.Output(); err != nil {
		return fmt.Errorf("go build: %v", err)
	}
	err = ioutil.WriteFile(filepath.Join(cfg.Path, cfg.PkgInfo.Name+".sh"), []byte(shRun), 0755)
	if err != nil {
		return
	}

	copyLib := func(lib string) (err error) {
		lib = strings.TrimPrefix(lib, "Qt")
		name := "libQt5" + lib + ".so"
		orig := filepath.Join(cfg.QtInfo.LibPath, name+"."+cfg.QtInfo.Version)
		targ := filepath.Join(cfg.Path, name+".5")
		err = copyFile(orig, targ)
		if err != nil {
			return
		}
		return
	}
	copyExtraLib := func(name string) (err error) {
		orig := filepath.Join(cfg.QtInfo.LibPath, name)
		targ := filepath.Join(cfg.Path, name)
		err = copyFile(orig, targ)
		if err != nil {
			return
		}
		return
	}

	if verbose {
		t.Log(logprefix, "copying libs")
	}
	for _, lib := range cfg.Profile.Libs["default"] {
		if err = copyLib(lib); err != nil {
			return
		}
	}
	for _, lib := range cfg.Profile.Libs["linux"] {
		if err = copyLib(lib); err != nil {
			return
		}
	}
	for _, lib := range cfg.Profile.Extra["linux"] {
		if err = copyExtraLib(lib); err != nil {
			return
		}
	}

	copyPlug := func(dir, name string) (err error) {
		name = "libq" + name + ".so"
		orig := filepath.Join(cfg.QtInfo.BasePath, "plugins", dir, name)
		targ := filepath.Join(cfg.Path, dir, name)
		if err = copyFile(orig, targ); err != nil {
			return
		}
		return
	}

	if verbose {
		t.Log(logprefix, "copying plugins")
	}
	err = os.Mkdir(filepath.Join(cfg.Path, "platforms"), 0755)
	if err != nil {
		return
	}
	for _, name := range cfg.Profile.Platforms["linux"] {
		if err = copyPlug("platforms", name); err != nil {
			return
		}
	}
	err = os.Mkdir(filepath.Join(cfg.Path, "imageformats"), 0755)
	if err != nil {
		return
	}
	for _, name := range cfg.Profile.Imageformats {
		if err = copyPlug("imageformats", name); err != nil {
			return
		}
	}

	copyMod := func(name string) (err error) {
		orig := filepath.Join(cfg.QtInfo.BasePath, name)
		targ := filepath.Join(cfg.Path, name)
		err = os.MkdirAll(filepath.Dir(targ), 0755)
		if err != nil {
			return
		}
		if err = copyTree(orig, targ); err != nil {
			return
		}
		return
	}

	if verbose {
		t.Log(logprefix, "copying modules")
	}
	err = copyTree(filepath.Join("project", "qml"), filepath.Join(cfg.Path, "qml"))
	if err != nil {
		return
	}
	for category, mods := range cfg.Profile.Modules {
		dir := filepath.Join(strings.Split(category, "/")...)
		for _, name := range mods {
			targ := filepath.Join(dir, name)
			if err = copyMod(targ); err != nil {
				return
			}
		}
	}
	err = ioutil.WriteFile(filepath.Join(cfg.Path, "qt.conf"), []byte(qtConfLinux), 0666)
	if err != nil {
		return
	}

	return
}

// deployWindows is a routine for Windows
func deployWindows(cfg *config, t *tasking.T) (err error) {
	logprefix := "deploy [windows]:"

	if verbose {
		t.Log(logprefix, "building executable")
	}
	name := filepath.Join(cfg.Path, cfg.PkgInfo.Name+".exe")
	cmd := exec.Command("go", "build", "-o", name, cfg.PkgInfo.ImportPath)
	if _, err = cmd.Output(); err != nil {
		return fmt.Errorf("go build: %v", err)
	}

	copyLib := func(lib string) (err error) {
		lib = strings.TrimPrefix(lib, "Qt")
		name := "Qt5" + lib + ".dll"
		orig := filepath.Join(cfg.QtInfo.BasePath, "bin", name)
		targ := filepath.Join(cfg.Path, name)
		err = copyFile(orig, targ)
		if err != nil {
			return
		}
		return
	}
	copyExtraLib := func(name string) (err error) {
		orig := filepath.Join(cfg.QtInfo.BasePath, "bin", name)
		targ := filepath.Join(cfg.Path, name)
		err = copyFile(orig, targ)
		if err != nil {
			return
		}
		return
	}

	if verbose {
		t.Log(logprefix, "copying libs")
	}
	for _, lib := range cfg.Profile.Libs["default"] {
		if err = copyLib(lib); err != nil {
			return
		}
	}
	for _, lib := range cfg.Profile.Libs["windows"] {
		if err = copyLib(lib); err != nil {
			return
		}
	}
	for _, lib := range cfg.Profile.Extra["windows"] {
		if err = copyExtraLib(lib); err != nil {
			return
		}
	}

	copyPlug := func(dir, name string) (err error) {
		name = "q" + name + ".dll"
		orig := filepath.Join(cfg.QtInfo.BasePath, "plugins", dir, name)
		targ := filepath.Join(cfg.Path, dir, name)
		if err = copyFile(orig, targ); err != nil {
			return
		}
		return
	}

	if verbose {
		t.Log(logprefix, "copying plugins")
	}
	err = os.Mkdir(filepath.Join(cfg.Path, "platforms"), 0755)
	if err != nil {
		return
	}
	for _, name := range cfg.Profile.Platforms["windows"] {
		if err = copyPlug("platforms", name); err != nil {
			return
		}
	}
	err = os.Mkdir(filepath.Join(cfg.Path, "imageformats"), 0755)
	if err != nil {
		return
	}
	for _, name := range cfg.Profile.Imageformats {
		if err = copyPlug("imageformats", name); err != nil {
			return
		}
	}

	copyMod := func(name string) (err error) {
		orig := filepath.Join(cfg.QtInfo.BasePath, name)
		targ := filepath.Join(cfg.Path, name)
		err = os.MkdirAll(filepath.Dir(targ), 0755)
		if err != nil {
			return
		}
		if err = copyTree(orig, targ); err != nil {
			return
		}
		return
	}

	if verbose {
		t.Log(logprefix, "copying modules")
	}
	err = copyTree(filepath.Join("project", "qml"), filepath.Join(cfg.Path, "qml"))
	if err != nil {
		return
	}
	for _, mods := range cfg.Profile.Modules {
		for _, name := range mods {
			targ := filepath.Join(name)
			if err = copyMod(targ); err != nil {
				return
			}
		}
	}
	return
}

// Parses output of `qmake -v` like
//
//     Using Qt version 5.2.1 in /usr/lib/x86_64-linux-gnu
//
// And `qtpaths --plugin-dir` like
//
//     /usr/local/Cellar/qt5/5.3.0/plugins
func getQtInfo() (info qtInfo, err error) {
	errFmt := fmt.Errorf("qt info: qmake unexpected output")

	buf, err := exec.Command("qmake", "-v").Output()
	if err != nil {
		err = fmt.Errorf("qt info: qmake: %v", err)
		return
	}
	idx := bytes.Index(buf, []byte("Qt version"))
	if idx < 0 {
		err = errFmt
		return
	}
	buf = buf[idx:]
	n, err := fmt.Sscanf(string(buf), "Qt version %s in %s", &info.Version, &info.LibPath)
	if err != nil || n != 2 {
		err = errFmt
	}

	// qtpaths
	buf, err = exec.Command("qtpaths", "--plugin-dir").Output()
	if err != nil {
		err = fmt.Errorf("qt info: qtpaths: %v", err)
		return
	}
	buf = bytes.TrimSpace(buf)
	if len(buf) < 1 || !bytes.HasSuffix(buf, []byte("plugins")) {
		err = fmt.Errorf("qt info: qtpaths unexpected output")
		return
	}
	buf = bytes.TrimSuffix(buf, []byte("plugins"))
	info.BasePath = string(buf[:len(buf)-1]) // drop sep
	return
}

// getPkgInfo fetches info about package being deployed.
func getPkgInfo() (info pkgInfo, err error) {
	buf, err := exec.Command("go", "list", "-f", "{{.ImportPath}}").Output()
	if err != nil {
		return
	}
	path := strings.TrimSpace(string(buf))
	info = pkgInfo{
		Name:       filepath.Base(path),
		ImportPath: path,
	}
	return
}

// darwinRelink makes paths of linked libraries relative to executable.
//
//   /usr/local/Cellar/qt5/5.3.0/lib/QtWidgets.framework/Versions/5/QtWidgets
//   /usr/local/opt/qt5/lib/QtWidgets.framework/Versions/5/QtWidgets
//   ->
//   @executable_path/../Frameworks/QtWidgets.framework/Versions/5/QtWidgets
func darwinRelink(qlib, name string, strict bool) (err error) {
	file, err := macho.Open(name)
	if err != nil {
		return
	}
	defer file.Close()
	libs, err := file.ImportedLibraries()
	if err != nil {
		return
	}
	var qlib2 string
	// detect alternative qlib (homebrew symlinks Qt to /usr/local/opt)
	for _, lib := range libs {
		idx := strings.Index(lib, "QtCore")
		if idx > 0 {
			qlib2 = lib[:idx-1] // drop sep
			break
		}
	}
	replacer := strings.NewReplacer(qlib, relinkBase, qlib2, relinkBase)
	if len(qlib2) < 1 && strict {
		return fmt.Errorf("darwin relink: corrupt binary: %s", name)
	} else if !strict {
		replacer = strings.NewReplacer(qlib, relinkBase)
	}
	// replace qlib/qlib2 to relinkBase
	for _, lib := range libs {
		rlib := replacer.Replace(lib)
		cmd := exec.Command("install_name_tool", "-change", lib, rlib, name)
		if err = cmd.Run(); err != nil {
			return fmt.Errorf("darwin relink: %v", err)
		}
	}
	return
}

// writeInfoPlist writes manifest for .app package to file.
func writeInfoPlist(path string, info pkgInfo) error {
	data := bundleInfo{
		Exec: info.Name,
		Id:   info.ImportPath,
	}
	infoPath := filepath.Join(path, "Info.plist")
	infoTpl := template.Must(template.New("info").Parse(infoPlist))
	file, err := os.Create(infoPath)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := infoTpl.Execute(file, data); err != nil {
		return err
	}
	return nil
}

// copyTree recursively copies orig dir into the targ dir.
func copyTree(orig, targ string) (err error) {
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		name, err := filepath.Rel(orig, path)
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			if strings.HasPrefix(info.Name(), ".") {
				return nil
			}
			err = copyFile(path, filepath.Join(targ, name))
		} else {
			err = os.Mkdir(filepath.Join(targ, name), 0755)
		}
		if err != nil {
			return err
		}
		return nil
	}
	if err = filepath.Walk(orig, walkFn); err != nil {
		return
	}
	return
}

// copyFile effectively copies a file orig to file targ.
func copyFile(orig, targ string) (err error) {
	in, err := os.Open(orig)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(targ)
	if err != nil {
		return
	}
	defer out.Close()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	return
}

const shRun = `#!/bin/sh
appname=` + "`basename $0 | sed s,\\.sh$,,`" + `
dirname=` + "`dirname $0`" + `
tmp="${dirname#?}"

if [ "${dirname%$tmp}" != "/" ]; then
dirname=$PWD/$dirname
fi
LD_LIBRARY_PATH=$dirname
export LD_LIBRARY_PATH
$dirname/$appname "$@"
`

const qtConfLinux = `[Paths]
Imports = qml
Qml2Imports = qml
`

const qtConfDarwin = `[Paths]
Imports = Resources/qml
Qml2Imports = Resources/qml
`

const infoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>NSPrincipalClass</key>
	<string>NSApplication</string>
	<key>CFBundleIconFile</key>
	<string>{{.Icon}}</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleGetInfoString</key>
	<string>Powered by Go QML</string>
	<key>CFBundleSignature</key>
	<string>????</string>
	<key>CFBundleExecutable</key>
	<string>{{.Exec}}</string>
	<key>CFBundleIdentifier</key>
	<string>{{.Id}}</string>
</dict>
</plist>
`
