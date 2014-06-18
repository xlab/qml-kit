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

/*
Package go-qml-kit represents a hello world Go/QML application that supports multiplatform deployment.

Also it can be a template for Qt Project, just click a few buttons and you're up with your new bootstrapped hello world.
It has specific structure providing ability to edit QML in QtCreator with no conflicts with Go code. It provides a task
for deployment automation, just write "gotask deploy" and your app is ready to publish (that means: single binary, some
resources were embedded in binary, some were copied to place, qt libs and modules were copied to place, paths are
correctly set up). Also there is work in progress about making resources embeddable. Images can be handled well already,
using engine.AddImageProvider, but there is a problem with QML files so we just copy them.

	.
	├── README.md
	├── deploy_profile.yaml
	├── deploy_task.go
	├── doc.go
	├── main.go
	├── project
	│   ├── images
	│   │   └── background.png
	│   ├── main.cpp
	│   ├── project.pro
	│   ├── qml
	│   │   └── main.qml
	│   └── qtquick2applicationviewer
	│       ├── qtquick2applicationviewer.cpp
	│       ├── qtquick2applicationviewer.h
	│       └── qtquick2applicationviewer.pri
	├── wizard.xml
	└── wizard_icon.png

Parts of this template can be used independently, for example you may wish to add a deployment task to your already
writen project — just copy `deploy_task.go` and `deploy_profile.yaml` files and run `gotask deploy`.

Installation

It's better to install that package as usual

	go get gopkg.in/qml-kit.v0

and then copy it around when you need to start a fresh project.

Qt Project Template

Also the package could be installed as Qt wizard template, make sure the target path is correct.

	git clone https://gopkg.in/qml-kit.v0 ~/.config/QtProject/qtcreator/templates/wizards/qml-kit.v0

Commands

A general help for each task can be received via help command

	gotask help <task>

Examples of tasks:

	gotask clean

Runs `rice clean` to remove leftovers from resource embedding, purges wizard configs if any left in project dir.
The `doc.go` file is being removed too, since it is not related to your aplication at all.

	gotask deploy -v

Runs deployment with verbosive output. You may use the `--dmg` option if you're running OS X. Make sure that all
of the used modules and libs are correctly listed in the `deploy_profile.yaml` manifest before you run deployment task.

Notes

This implementation uses `qmake` and `qtpaths` in order to detect the paths Qt is installed, so make sure you've set
`$PATH` correctly so these binaries are related to Qt distribution you're linking against. It's easy to check by comparing
outputs from `qmake -v` and `pkg-config --cflags`.

Another important thing is that the Qt libs shipped with ubuntu are not suitable for deploying since `libqxcb.so` platform
contains too many linked libs. Compare these `ldd` outputs: http://pastebin.com/nVeg2eGQ (5.2.1+dfsg-1ubuntu14.2)
and http://pastebin.com/vtWbbAwZ (5.3.0 distribution). So we recommend you to download and install the official Qt distribution.
*/
package main
