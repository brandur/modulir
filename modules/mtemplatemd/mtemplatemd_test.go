package mtemplatemd

import (
	"context"
	"os"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestIncludeMarkdown(t *testing.T) {
	content := []byte("**hello, world**")
	tmpfile, err := os.CreateTemp("", "markdown_sample.md")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write(content)
	assert.NoError(t, err)

	err = tmpfile.Close()
	assert.NoError(t, err)

	ctx, container := Context(context.Background())

	assert.Equal(t, `<p><strong>hello, world</strong></p>`,
		strings.TrimSpace(string(IncludeMarkdown(ctx, tmpfile.Name()))))

	assert.Contains(t, container.dependenciesMap, tmpfile.Name())
	assert.Contains(t, container.Dependencies, tmpfile.Name())
}
