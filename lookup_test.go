package utr

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultKeeper(t *testing.T) {
	wrongHostname := "/" + testHostname

	require.Error(t, AddPath(wrongHostname, testSocketPath))
	require.NoError(t, AddPath(testHostname, testSocketPath))
	require.NoError(t, AddPath(testHostname, testSocketPath))
	require.Error(t, AddPath(testHostname, filepath.Join("dir", testSocketPath)))

	path, err := LookupPath(wrongHostname)
	require.Error(t, err)
	require.Empty(t, path)

	path, err = LookupPath(testHostname)
	require.NoError(t, err)
	require.Equal(t, testSocketPath, path)
}
