package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"

	"bitbucket.org/kardianos/osext"
	"github.com/GeertJohan/go.rice"
	"gopkg.in/qml.v1"
)

var imageEmpty = image.NewRGBA(image.Rect(0, 0, 16, 16))
var boxImages *rice.Box
var boxQml *rice.Box

func main() {
	if err := qml.Run(run); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	boxImages = rice.MustFindBox("project/images")
	boxQml = rice.MustFindBox("project/qml")

	engine := qml.NewEngine()

	engine.AddImageProvider("images", unboxImage)

	engine.On("quit", func() {
		fmt.Println("qml quit")
		os.Exit(0)
	})

	component, err := componentFromFile("main.qml", engine)
	if err != nil {
		return err
	}
	win := component.CreateWindow(nil)

	win.Show()
	win.Wait()

	return nil
}

// The unboxImage function is an image provider used within engine.AddImageProvider,
// loads image resources from the the specified rice box.
func unboxImage(name string, width, height int) image.Image {
	file, err := boxImages.Open(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resource: %v\n", err)
		return imageEmpty
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return imageEmpty
	}
	return img
}

// The componentFromFile function finds an existing path of qml file with given name
// and creates a component from it. This is supposed to be replaced with a rice box soon.
func componentFromFile(name string, engine *qml.Engine) (qml.Object, error) {
	prefix, err := qmlPrefix()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(prefix, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err0 := err
		path = filepath.Join("project", "qml", name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil, err0
		}
	}
	component, err := engine.LoadFile(path)
	if err != nil {
		return nil, err
	}
	return component, nil
}

// The qmlPrefix function returns an executable-related path of dir with qml files.
func qmlPrefix() (path string, err error) {
	path, err = osext.ExecutableFolder()
	if err != nil {
		return
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(path, "..", "Resources", "qml"), nil
	default:
		return filepath.Join(path, "qml"), nil
	}
}
