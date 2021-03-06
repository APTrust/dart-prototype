package fileutil_test

import (
	"github.com/APTrust/dart/util"
	"github.com/APTrust/dart/util/fileutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func getTestDataPath() string {
	_, filename, _, _ := runtime.Caller(0)
	testDataPath, _ := filepath.Abs(path.Join(filepath.Dir(filename), "..", "..", "testdata"))
	if runtime.GOOS == "windows" {
		// WTF??
		testDataPath, _ = filepath.Abs(path.Join(filepath.Dir(filename), "..", "..", "..", "testdata"))
	}
	return testDataPath
}

func TestNewFileSystemIterator(t *testing.T) {
	testDataPath := getTestDataPath()
	fsi, err := fileutil.NewFileSystemIterator(testDataPath)
	assert.Nil(t, err)
	assert.NotNil(t, fsi)

	badPath := path.Join(testDataPath, "path", "does", "not", "exist")
	fsi, err = fileutil.NewFileSystemIterator(badPath)
	assert.NotNil(t, err)
	assert.Nil(t, fsi)
	assert.True(t, strings.Contains(err.Error(), "does not exist"))

	badPath = "not/an/absolute/path"
	fsi, err = fileutil.NewFileSystemIterator(badPath)
	assert.NotNil(t, err)
	assert.Nil(t, fsi)
	assert.True(t, strings.Contains(err.Error(), "must be absolute"))

	_, filename, _, _ := runtime.Caller(0)
	fsi, err = fileutil.NewFileSystemIterator(filename)
	assert.NotNil(t, err)
	assert.Nil(t, fsi)
	assert.True(t, strings.Contains(err.Error(), "is not a directory"))
}

func TestFSINext(t *testing.T) {
	fsi, err := fileutil.NewFileSystemIterator(getTestDataPath())
	require.Nil(t, err)
	if fsi == nil {
		assert.Fail(t, "Could not get a FileSystemIterator")
	}
	for {
		reader, fileSummary, err := fsi.Next()
		if reader != nil {
			defer reader.Close()
		}
		if err == io.EOF {
			break
		}
		require.NotNil(t, fileSummary)
		assert.NotEmpty(t, fileSummary.RelPath)
		assert.False(t, strings.HasPrefix(fileSummary.RelPath, string(os.PathSeparator)))
		assert.NotNil(t, fileSummary.Mode)
		assert.True(t, fileSummary.Size > int64(0))
		assert.False(t, fileSummary.ModTime.IsZero())
		// This will have to change if we have subdirs under testdata
		assert.False(t, fileSummary.IsDir)

		buf := make([]byte, 1024)
		_, err = reader.Read(buf)
		if err != nil {
			assert.Equal(t, io.EOF, err)
		}
	}
}

func TestFSIGetTopLevelDirNames(t *testing.T) {
	fsi, _ := fileutil.NewFileSystemIterator(getTestDataPath())
	if fsi == nil {
		assert.Fail(t, "Could not get a FileSystemIterator")
	}
	names := fsi.GetTopLevelDirNames()
	require.NotEmpty(t, names)
	assert.Equal(t, 1, len(names))
	assert.Equal(t, "testdata", names[0])
}

func TestFSIOpenFile(t *testing.T) {
	fsi, _ := fileutil.NewFileSystemIterator(getTestDataPath())
	if fsi == nil {
		assert.Fail(t, "Could not get a FileSystemIterator")
	}
	file, err := fsi.OpenFile(filepath.Join("bags", "example.edu.tagsample_good.tar"))
	if file != nil {
		defer file.Close()
	}
	assert.Nil(t, err)
	assert.NotNil(t, file)

	file2, err := fsi.OpenFile(filepath.Join("bags", "does_not_exist.tar"))
	if file2 != nil {
		defer file2.Close()
	}
	assert.NotNil(t, err)
	assert.Nil(t, file2)
}

func TestFSIFindMatchingFiles(t *testing.T) {
	fsi, _ := fileutil.NewFileSystemIterator(getTestDataPath())
	if fsi == nil {
		assert.Fail(t, "Could not get a FileSystemIterator")
	}

	reJsonFile := regexp.MustCompile(".*\\.json$")
	fileNames, err := fsi.FindMatchingFiles(reJsonFile)
	require.Nil(t, err)
	require.Equal(t, 4, len(fileNames))
	assert.True(t, util.StringListContains(fileNames, filepath.Join("profiles", "aptrust_bagit_profile_2.2.json")))
	assert.True(t, util.StringListContains(fileNames, filepath.Join("profiles", "dpn_bagit_profile_2.1.json")))
	assert.True(t, util.StringListContains(fileNames, filepath.Join("jobs", "sample_job.json")))

	reTarFile := regexp.MustCompile(".*\\.tar$")
	fileNames, err = fsi.FindMatchingFiles(reTarFile)
	require.Nil(t, err)
	require.Equal(t, 17, len(fileNames))
}
