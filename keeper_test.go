package utr

import (
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeeper(t *testing.T) {
	const (
		wrongHostname       = "/" + testHostname
		nonexistentHostname = testHostname + testHostname
	)

	var keeper Keeper

	require.Error(t, keeper.AddPath(wrongHostname, testSocketPath))
	require.NoError(t, keeper.AddPath(testHostname, testSocketPath))
	require.NoError(t, keeper.AddPath(testHostname, testSocketPath))
	require.Error(t, keeper.AddPath(testHostname, filepath.Join("dir", testSocketPath)))

	path, err := keeper.LookupPath(nonexistentHostname)
	require.Error(t, err)
	require.Empty(t, path)

	path, err = keeper.LookupPath(testHostname)
	require.NoError(t, err)
	require.Equal(t, testSocketPath, path)
}

func BenchmarkAddPathReference(b *testing.B) {
	table := make(map[string]string)

	for id := range b.N {
		hostname := testHostname + strconv.Itoa(id)

		table[hostname] = testSocketPath
	}
}

func BenchmarkAddPathKeeper(b *testing.B) {
	var keeper Keeper

	for id := range b.N {
		hostname := testHostname + strconv.Itoa(id)

		if err := keeper.AddPath(hostname, testSocketPath); err != nil {
			require.NoError(b, err)
		}
	}
}

func BenchmarkLookupPathReference(b *testing.B) {
	table := make(map[string]string)

	table[testHostname] = testSocketPath

	for b.Loop() {
		if path := table[testHostname]; path != testSocketPath {
			require.Equal(b, testSocketPath, path)
		}
	}
}

func BenchmarkLookupPathKeeper(b *testing.B) {
	var keeper Keeper

	require.NoError(b, keeper.AddPath(testHostname, testSocketPath))

	for b.Loop() {
		path, err := keeper.LookupPath(testHostname)
		if err != nil {
			require.NoError(b, err)
		}

		if path != testSocketPath {
			require.Equal(b, testSocketPath, path)
		}
	}
}

func BenchmarkRaceKeeper(b *testing.B) {
	var (
		keeper  Keeper
		counter atomic.Int64
	)

	require.NoError(b, keeper.AddPath(testHostname, testSocketPath))

	b.ResetTimer()

	b.RunParallel(
		func(pb *testing.PB) {
			for pb.Next() {
				id := counter.Add(1)

				if id%2 == 0 {
					if _, err := keeper.LookupPath(testHostname); err != nil {
						require.NoError(b, err)
					}

					continue
				}

				hostname := testHostname + strconv.FormatInt(id, 10)

				if err := keeper.AddPath(hostname, testSocketPath); err != nil {
					require.NoError(b, err)
				}
			}
		},
	)
}
