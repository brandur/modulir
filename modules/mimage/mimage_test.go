package mimage

import (
	"io/ioutil"
	"os"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func init() {
	MagickBin = os.Getenv("MAGICK_BIN")
	if MagickBin == "" {
		panic("set MAGICK_BIN env to the location of ImageMagick")
	}

	MozJPEGBin = os.Getenv("MOZ_JPEG_BIN")
	PNGQuantBin = os.Getenv("PNGQUANT_BIN")
}

func TestResizeImageJPEG(t *testing.T) {
	d, _ := os.Getwd()
	t.Logf("pwd = %v\n", d)

	tmpfile, err := ioutil.TempFile("", "resized_image")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	err = resizeImage(nil, "./samples/square.jpg", tmpfile.Name(),
		100, nil, PhotoGravityCenter)
	assert.NoError(t, err)
}

func TestResizeImageJPEG_NoMozJPEG(t *testing.T) {
	if MozJPEGBin == "" {
		return
	}

	tmpfile, err := ioutil.TempFile("", "resized_image")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	err = resizeImage(nil, "./samples/square.jpg", tmpfile.Name(),
		100, nil, PhotoGravityCenter)
	assert.NoError(t, err)
}

func TestResizeImagePNG(t *testing.T) {
	d, _ := os.Getwd()
	t.Logf("pwd = %v\n", d)

	tmpfile, err := ioutil.TempFile("", "resized_image")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	err = resizeImage(nil, "./samples/sample.png", tmpfile.Name(),
		100, nil, PhotoGravityCenter)
	assert.NoError(t, err)
}
