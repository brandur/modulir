package mimage

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/brandur/modulir"
	"github.com/brandur/modulir/modules/mfile"
	gocache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
)

//////////////////////////////////////////////////////////////////////////////
//
//
//
// Public
//
//
//
//////////////////////////////////////////////////////////////////////////////

// MagickBin is the location of the `magick` binary that ships with the
// ImageMagick project (an image manipulation utility).
//
// Must be configured to use this package.
var MagickBin string

// MozJPEGBin is the location of the `cjpeg` binary that ships with the mozjpeg
// project (a JPG optimizer). If configured, JPEGs are passed through an
// optimization pass after resizing them.
var MozJPEGBin string

// PNGQuantBin is the location of the `pnqquant` binary (a PNG optimizer). If
// configured, PNGs are passed through an optimization pass after resizing
// them.
var PNGQuantBin string

// PhotoCropSettings are directives on how the image should be cropped
// depending on its proportions.
type PhotoCropSettings struct {
	// Square defines the crop ratio that will be used if the photo is square.
	//
	// Should be a string like "3:2", or empty for no crop.
	Square string

	// Landscape defines the crop ratio that will be used if the photo's width
	// is greater than its height.
	//
	// Should be a string like "3:2", or empty for no crop.
	Landscape string

	// Portrait defines the crop ratio that will be used if the photo's height
	// is greater than its width.
	//
	// Should be a string like "3:2", or empty for no crop.
	Portrait string
}

// PhotoGravity is the crop gravity for ImageMagick.
type PhotoGravity string

// Possible options for photo crop gravity.
const (
	PhotoGravityCenter    PhotoGravity = "center"
	PhotoGravityEast      PhotoGravity = "east"
	PhotoGravityNorth     PhotoGravity = "north"
	PhotoGravityNorthEast PhotoGravity = "northeast"
	PhotoGravityNorthWest PhotoGravity = "northwest"
	PhotoGravitySouth     PhotoGravity = "south"
	PhotoGravitySouthEast PhotoGravity = "southeast"
	PhotoGravitySouthWest PhotoGravity = "southwest"
	PhotoGravityWest      PhotoGravity = "west"
)

// PhotoSize are the specifications for a target photo crop and resize.
type PhotoSize struct {
	Suffix       string
	Width        int
	CropSettings *PhotoCropSettings
}

// FetchAndResizeImage fetches an image from a URL and resizes it according to
// specifications.
func FetchAndResizeImage(c *modulir.Context,
	u *url.URL, targetDir, targetSlug, tempDir string,
	cropGravity PhotoGravity, photoSizes []PhotoSize) (bool, error) {

	// source without an extension, e.g. `content/photographs/123`
	sourceNoExt := filepath.Join(targetDir, targetSlug)

	// A "marker" is an empty file that we commit to a photograph directory
	// that indicates that we've already done the work to fetch and resize a
	// photo. It allows us to skip duplicate work even if we don't have the
	// work's results available locally. This is important for CI where we
	// store results to an S3 bucket, but don't pull them all back down again
	// for every build.
	markerPath := sourceNoExt + ".marker"

	// We use an in-memory cache to store whether markers exist for some period
	// of time because going to the filesystem to check every one of them is
	// relatively slow/expensive.
	if _, ok := photoMarkerCache.Get(markerPath); ok {
		c.Log.Debugf("Skipping photo fetch + resize because marker cached: %s",
			markerPath)
		return false, nil
	}

	// Otherwise check the filesystem.
	if mfile.Exists(markerPath) {
		c.Log.Debugf("Skipping photo fetch + resize because marker exists: %s",
			markerPath)
		photoMarkerCache.Set(markerPath, struct{}{}, gocache.DefaultExpiration)
		return false, nil
	}

	// Create a target output directory if necessary. This is only used for
	// "other" photographs (not part of the main series) which may specify a
	// subdirectory.
	if fullTargetDir := path.Dir(sourceNoExt); targetDir != path.Clean(targetDir) {
		err := mfile.EnsureDir(c, fullTargetDir)
		if err != nil {
			return true, err
		}
	}

	ext := strings.ToLower(filepath.Ext(u.Path))
	originalPath := filepath.Join(tempDir, targetSlug+"_original"+ext)
	if fullTempDir := path.Dir(originalPath); fullTempDir != path.Clean(tempDir) {
		err := mfile.EnsureDir(c, fullTempDir)
		if err != nil {
			return true, err
		}
	}

	err := fetchData(c, u, originalPath)
	if err != nil {
		return true, errors.Wrapf(err, "Error fetching image: %s", targetSlug)
	}

	for _, size := range photoSizes {
		err := resizeImage(c, originalPath,
			sourceNoExt+size.Suffix+ext, size.Width, size.CropSettings, cropGravity)
		if err != nil {
			return true, errors.Wrapf(err, "Error resizing image: %s", targetSlug)
		}
	}

	// After everything is done, created a marker file to indicate that the
	// work doesn't need to be redone.
	file, err := os.OpenFile(markerPath, os.O_RDONLY|os.O_CREATE, 0755)
	if err != nil {
		return true, errors.Wrapf(err, "Error creating marker for image: %s", targetSlug)
	}
	file.Close()

	return true, nil
}

//////////////////////////////////////////////////////////////////////////////
//
//
//
// Private
//
//
//
//////////////////////////////////////////////////////////////////////////////

// An expiring cache that tracks the current state of marker files for photos.
// Going to the filesystem on every build loop is relatively slow/expensive, so
// this helps speed up the build loop.
//
// Arguments are (defaultExpiration, cleanupInterval).
var photoMarkerCache = gocache.New(5*time.Minute, 10*time.Minute)

// fetchData is a helper for fetching a file via HTTP and storing it the local
// filesystem.
func fetchData(c *modulir.Context, u *url.URL, target string) error {
	c.Log.Debugf("Fetching file: %v", u.String())

	resp, err := http.Get(u.String())
	if err != nil {
		return errors.Wrapf(err, "Error fetching: %v", u.String())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("Unexpected status code fetching '%v': %d",
			u.String(), resp.StatusCode)
	}

	f, err := os.Create(target)
	if err != nil {
		return errors.Wrapf(err, "Error creating: %v", target)
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	// probably not needed
	defer w.Flush()

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return errors.Wrapf(err, "Error copying to '%v' from HTTP response",
			target)
	}

	return nil
}

func resizeImage(c *modulir.Context,
	source, target string, width int, cropSettings *PhotoCropSettings, cropGravity PhotoGravity) error {

	if MagickBin == "" {
		return fmt.Errorf("mimage.MagickBin must be configured for image resizing")
	}

	out, err := exec.Command(
		MagickBin,
		"convert",
		source,
		"-auto-orient",
		"-format",
		"%[w] %[h]",
		"info:",
	).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "Error running convert info command (out: '%s')",
			string(out))
	}

	dimensions := strings.Split(string(out), " ")

	imageWidth, err := strconv.Atoi(dimensions[0])
	if err != nil {
		return errors.Wrapf(err, "Error converting width '%s' to integer", dimensions[0])
	}

	imageHeight, err := strconv.Atoi(dimensions[1])
	if err != nil {
		return errors.Wrapf(err, "Error converting height '%s' to integer", dimensions[1])
	}

	isSquare := imageWidth == imageHeight
	isLandscape := imageWidth > imageHeight
	isPortrait := imageWidth < imageHeight

	var resizeErrOut bytes.Buffer
	var optimizeErrOut bytes.Buffer

	// This is a little awkward, but we start out with some shared arguments,
	// add a few conditional ones based on landscape versus portrait, then add
	// a few more shared arguments. The order of the pipeline is important in
	// ImageMagick, so this is necessary.
	resizeArgs := []string{
		MagickBin,
		"convert",
		source,
		"-auto-orient",
		"-gravity",
		string(cropGravity),
	}

	if cropSettings != nil {
		if isSquare && cropSettings.Square != "" {
			resizeArgs = append(
				resizeArgs,
				"-crop",
				cropSettings.Square,
			)
		} else if isLandscape && cropSettings.Landscape != "" {
			resizeArgs = append(
				resizeArgs,
				"-crop",
				cropSettings.Landscape,
			)
		} else if isPortrait && cropSettings.Portrait != "" {
			resizeArgs = append(
				resizeArgs,
				"-crop",
				cropSettings.Portrait,
			)
		}
	}

	resizeArgs = append(
		resizeArgs,
		"-resize",
		fmt.Sprintf("%vx", width),
		"-quality",
		"85",
	)

	ext := strings.ToLower(filepath.Ext(source))

	// If we have mozjpeg then output to stdout and let it take in the resized
	// JPEG via pipe. Some for PNG. If not, then just resize to the target file
	// immediately.
	if ext == ".jpg" && MozJPEGBin != "" {
		resizeArgs = append(resizeArgs, "JPEG:-")
	} else if ext == ".png" && PNGQuantBin != "" {
		resizeArgs = append(resizeArgs, "PNG:-")
	} else {
		resizeArgs = append(resizeArgs, target)
	}

	resizeCmd := exec.Command(resizeArgs[0], resizeArgs[1:]...)
	resizeCmd.Stderr = &resizeErrOut

	var optimizeCmd *exec.Cmd
	r, w := io.Pipe()
	if ext == ".jpg" && MozJPEGBin != "" {
		optimizeCmd = exec.Command(
			MozJPEGBin,
			"-optimize",
			"-outfile",
			target,
			"-progressive",
		)
	} else if ext == ".png" && PNGQuantBin != "" {
		optimizeCmd = exec.Command(
			PNGQuantBin,
			"--output",
			target,
			"-",
		)
	}

	if optimizeCmd != nil {
		optimizeCmd.Stderr = &optimizeErrOut

		resizeCmd.Stdout = w
		optimizeCmd.Stdin = r
	}

	if err := resizeCmd.Start(); err != nil {
		return errors.Wrapf(err, "Error starting resize command")
	}

	if optimizeCmd != nil {
		if err := optimizeCmd.Start(); err != nil {
			return errors.Wrapf(err, "Error starting optimize command")
		}
	}

	if err := resizeCmd.Wait(); err != nil {
		return fmt.Errorf("%v (stderr: %v)", err, resizeErrOut.String())
	}

	w.Close()

	if MozJPEGBin != "" {
		if err := optimizeCmd.Wait(); err != nil {
			return fmt.Errorf("%v (stderr: %v)", err, optimizeErrOut.String())
		}
	}

	return nil
}
