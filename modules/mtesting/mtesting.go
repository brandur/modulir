package mtesting

import (
	"io/ioutil"
	"testing"

	assert "github.com/stretchr/testify/require"

	"github.com/brandur/modulir"
)

// NewContext is a convenience helper to create a new modulir.Context suitable
// for use in the test suite.
func NewContext() *modulir.Context {
	return modulir.NewContext(&modulir.Args{Log: &modulir.Logger{Level: modulir.LevelInfo}})
}

// WriteTempFile writes the given data to a temporary file. It returns the path
// to the temporary file which should be removed with `defer os.Remove(path)`.
func WriteTempFile(t *testing.T, data []byte) string {
	t.Helper()

	tempFile, err := ioutil.TempFile("", "modulir")
	assert.NoError(t, err)

	_, err = tempFile.Write(data)
	assert.NoError(t, err)

	err = tempFile.Close()
	assert.NoError(t, err)

	return tempFile.Name()
}
