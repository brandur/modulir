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

	MozJPEGBin = os.Getenv("MOZJPEG_BIN")
	PNGQuantBin = os.Getenv("PNGQUANT_BIN")
}

func TestResizeImageJPEG(t *testing.T) {
	if MozJPEGBin == "" {
		t.Logf("MOZ_JPEG_BIN not set; skipping full JPEG resize test")
		return
	}

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
	oldBin := MozJPEGBin
	MozJPEGBin = ""
	defer func() {
		MozJPEGBin = oldBin
	}()

	d, _ := os.Getwd()
	t.Logf("pwd = %v\n", d)

	tmpfile, err := ioutil.TempFile("", "resized_image")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	err = resizeImage(nil, "./samples/square.jpg", tmpfile.Name(),
		100, nil, PhotoGravityCenter)
	assert.NoError(t, err)
}

func TestResizeImagePNG(t *testing.T) {
	if MozJPEGBin == "" {
		t.Logf("PNGQUANT_BIN not set; skipping full PNG resize test")
		return
	}

	d, _ := os.Getwd()
	t.Logf("pwd = %v\n", d)

	tmpfile, err := ioutil.TempFile("", "resized_image")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	err = resizeImage(nil, "./samples/sample.png", tmpfile.Name(),
		100, nil, PhotoGravityCenter)
	assert.NoError(t, err)
}

func TestResizeImagePNG_NoPNGQuant(t *testing.T) {
	oldBin := PNGQuantBin
	PNGQuantBin = ""
	defer func() {
		PNGQuantBin = oldBin
	}()

	d, _ := os.Getwd()
	t.Logf("pwd = %v\n", d)

	tmpfile, err := ioutil.TempFile("", "resized_image")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	err = resizeImage(nil, "./samples/sample.png", tmpfile.Name(),
		100, nil, PhotoGravityCenter)
	assert.NoError(t, err)
}
