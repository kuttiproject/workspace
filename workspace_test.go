package workspace_test

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"testing"

	"github.com/kuttiproject/kuttilog"
	"github.com/kuttiproject/workspace"
)

const tsubdirname = "testsubdir"

func TestSet(t *testing.T) {
	defer workspace.Reset()

	// Set workspace, check kutti-config and kutti-reset subdirectories
	tdir := t.TempDir()
	wdir := path.Join(tdir, "wksp1")
	confdir := path.Join(wdir, "kutti-config")
	cachedir := path.Join(wdir, "kutti-cache")
	confsubdir := path.Join(confdir, tsubdirname)
	cachesubdir := path.Join(cachedir, tsubdirname)

	t.Logf("Setting workspace to %v", wdir)
	workspace.Set(wdir)

	err := checkdir(wdir)
	if err != nil {
		t.Errorf(
			"Setting workdir to %v failed: %v",
			wdir,
			err,
		)
	}

	checkdirfunc(t, confdir, "Configdir", workspace.Configdir)

	checkdirfunc(t, cachedir, "Cachedir", workspace.Cachedir)

	checkdirfunc(t, confsubdir, "Configsubdir", func() (string, error) {
		return workspace.Configsubdir(tsubdirname)
	})

	checkdirfunc(t, cachesubdir, "Cachesubdir", func() (string, error) {
		return workspace.Cachesubdir(tsubdirname)
	})
}

func TestReset(t *testing.T) {
	// Reset workspace, check UserConfigDir/kutti and UserCacheDir/kutti
	t.Logf("Resetting workspace")
	workspace.Reset()

	confdir, _ := os.UserConfigDir()
	confdir = path.Join(confdir, "kutti")
	cachedir, _ := os.UserCacheDir()
	cachedir = path.Join(cachedir, "kutti")
	confsubdir := path.Join(confdir, tsubdirname)
	cachesubdir := path.Join(cachedir, tsubdirname)

	checkdirfunc(t, confdir, "Post-reset Configdir", workspace.Configdir)

	checkdirfunc(t, cachedir, "Post-reset Cachedir", workspace.Cachedir)

	checkdirfunc(t, confsubdir, "Post-reset Configsubdir", func() (string, error) {
		return workspace.Configsubdir(tsubdirname)
	})

	checkdirfunc(t, cachesubdir, "Post-reset Cachesubdir", func() (string, error) {
		return workspace.Cachesubdir(tsubdirname)
	})

	os.RemoveAll(confsubdir)
	os.RemoveAll(cachesubdir)
}

func TestSetWithPopulatedDirectory(t *testing.T) {
	defer workspace.Reset()

	// Set workspace to prepoulated directory, and check failure
	tdir := t.TempDir()
	wdir := path.Join(tdir, "wksp1")
	confdir := path.Join(wdir, "kutti-config")
	cachedir := path.Join(wdir, "kutti-cache")

	// Set workspace to populated directory
	t.Logf("Setting workspace to %v", wdir)
	workspace.Set(wdir)

	checkpopulateddirfunc(t, confdir, "Configdir", workspace.Configdir)

	checkpopulateddirfunc(t, cachedir, "Cachedir", workspace.Cachedir)

	_, err := workspace.Configdir()
	if err != nil {
		t.Errorf("Post-cleanup Configdir failed with: %v", err)
	}

	_, err = workspace.Cachedir()
	if err != nil {
		t.Errorf("Post-cleanup Cachedir failed with: %v", err)
	}

	confsubdir := path.Join(confdir, tsubdirname)
	cachesubdir := path.Join(cachedir, tsubdirname)

	checkpopulateddirfunc(t, confsubdir, "Configsubdir", func() (string, error) {
		return workspace.Configsubdir(tsubdirname)
	})

	checkpopulateddirfunc(t, cachesubdir, "Cachesubdir", func() (string, error) {
		return workspace.Cachesubdir(tsubdirname)
	})
}

func TestWithNoPermissions(t *testing.T) {
	defer workspace.Reset()

	tdir := t.TempDir()
	wpath := path.Join(tdir, "wksp1")
	err := os.Mkdir(wpath, 0)
	if err != nil {
		t.Logf(
			"Workspace directory creation failed with: %v",
			err,
		)
		t.FailNow()
	}

	err = workspace.Set(wpath)
	if err != nil {
		t.Logf("Error on trying to set workspace: %v", err)
		t.Fail()
	}

	_, err = workspace.Configdir()
	if err == nil {
		t.Logf("There should have been an error getting config directory.")
		t.Fail()
	}

	_, err = workspace.Cachedir()
	if err == nil {
		t.Logf("There should have been an error getting cache directory.")
		t.Fail()
	}

	_, err = workspace.Configsubdir(tsubdirname)
	if err == nil {
		t.Logf("There should have been an error getting config subdirectory.")
		t.Fail()
	}

	_, err = workspace.Cachesubdir(tsubdirname)
	if err == nil {
		t.Logf("There should have been an error getting cache subdirectory.")
		t.Fail()
	}

	wpath = path.Join(wpath, "subwksp")
	err = workspace.Set(wpath)
	if err == nil {
		t.Logf("There should have been an error setting workspace directory.")
		t.Fail()
	}
}

func checkdir(dirpath string) error {
	dirinfo, err := os.Stat(dirpath)
	if err == nil && !dirinfo.IsDir() {
		err = errors.New("not a directory")
	}
	return err
}

func checkdirfunc(t *testing.T, expectedresult string, funcname string, f func() (string, error)) {
	funcresult, funcerr := f()
	if funcresult != expectedresult ||
		funcerr != nil {

		t.Errorf(
			"%v failed. Expected %v, got %v with error %v",
			funcname,
			expectedresult,
			funcresult,
			funcerr,
		)
	}
}

func checkpopulateddirfunc(t *testing.T, dirpath string, funcname string, f func() (string, error)) {
	// Set up files instead of directories
	dirblockfile, err := os.Create(dirpath)
	if err != nil {
		t.Errorf("Could not create temporary file %v: %v", dirpath, err)
	}
	dirblockfile.Close()

	_, err = f()
	if err == nil {
		t.Errorf("%v in populated directory should have failed.", funcname)
	}

	// Clean up files
	os.Remove(dirpath)
}

// File config manager tests
type sampledata struct {
	Name string
	Age  int
}

func (sd *sampledata) Serialize() ([]byte, error) {
	return json.Marshal(sd)
}

func (sd *sampledata) Deserialize(data []byte) error {
	var loadedconfig sampledata
	err := json.Unmarshal(data, &loadedconfig)
	if err == nil {
		sd.Name = loadedconfig.Name
		sd.Age = loadedconfig.Age
	}
	return err
}

func (sd *sampledata) Setdefaults() {
	*sd = sampledata{
		Name: "Test",
		Age:  42,
	}
}

func TestFileConfigManager(t *testing.T) {
	kuttilog.Setloglevel(kuttilog.Debug)

	tdir := t.TempDir()
	workspace.Set(tdir)
	defer workspace.Reset()

	config := &sampledata{}

	// Test NewFileConfigManager
	fcm, err := workspace.NewFileConfigmanager("testfile.json", config)
	if err != nil {
		t.Logf("Error while getting new ConfigManager: %v", err)
		t.FailNow()
	}

	err = fcm.Load()
	if err != nil {
		t.Logf("Error while loading new ConfigManager: %v", err)
		t.FailNow()
	}

	if config.Name != "Test" || config.Age != 42 {
		t.Logf("ConfigManager.Load() failed to set default values. Values are: %#v", config)
		t.Fail()
	}

	config.Name = "Test2"
	config.Age = 47

	err = fcm.Save()
	if err != nil {
		t.Logf("ConfigManager.Save() failed with: %v", err)
		t.FailNow()
	}

	// Test Load()
	fcm.Reset()

	err = fcm.Load()
	if err != nil {
		t.Logf("Error while loading ConfigManager: %v", err)
		t.FailNow()
	}

	if config.Name != "Test2" || config.Age != 47 {
		t.Logf("ConfigManager.Load() failed to retrieve values. Values are: %#v", config)
		t.Fail()
	}

	// Test Load failure
	fcm.Reset()

	//confdir, err := workspace.Configdir()
	os.RemoveAll(tdir)
	if err != nil {
		t.Logf("Removing directory returned error: %v", err)
		t.FailNow()
	}

	err = fcm.Load()
	if err == nil {
		t.Log("Load should have errored out here.")
		t.Fail()
	}

}

// Test file utilities
func TestChecksum(t *testing.T) {
	result, err := workspace.ChecksumFile("workspace_test.go")
	if err != nil {
		t.Logf("Checksum failed with error:%v", err)
		t.FailNow()
	}
	t.Logf("Checksum is '%v'", result)
}

func TestCopyFile(t *testing.T) {
	sourceresult, err := workspace.ChecksumFile("workspace_test.go")
	if err != nil {
		t.Logf("Checksum source failed with error:%v", err)
		t.FailNow()
	}
	t.Logf("Source Checksum is '%v'", sourceresult)

	err = workspace.CopyFile("workspace_test.go", "deletethis_test.xxx", 1000, true)
	if err != nil {
		t.Logf("Copyfile failed with error:%v", err)
		t.FailNow()
	}

	defer os.Remove("deletethis_test.xxx")

	destresult, err := workspace.ChecksumFile("deletethis_test.xxx")
	if err != nil {
		t.Logf("Checksum of destination failed with error:%v", err)
		t.FailNow()
	}
	t.Logf("Destination Checksum is '%v'", destresult)

	if destresult != sourceresult {
		t.Log("Source and destination checksums don't match. Copy was faulty.")
		t.FailNow()
	}

	// Try failing the copy due to destination being present
	err = workspace.CopyFile("workspace_test.go", "deletethis_test.xxx", 1000, false)
	if err == nil {
		t.Log("Copying without overwrite should have caused an error.")
		t.Fail()
	}

}

func TestDownloadFile(t *testing.T) {
	const dfilename = "soedit-1.0.tar.gz"
	err := workspace.DownloadFile(
		"https://github.com/rajch/soedit/releases/download/v1.0/soedit-1.0.tar.gz",
		dfilename,
	)
	if err != nil {
		t.Logf("Downloadfile failed with error:%v\n", err)
		t.FailNow()
	}

	defer workspace.RemoveFile(dfilename)

	destresult, err := workspace.ChecksumFile(dfilename)
	if err != nil {
		t.Logf("Checksum of destination failed with error:%v", err)
		t.FailNow()
	}
	t.Logf("Destination Checksum is '%v'", destresult)

	if destresult != "1f7960f2a6629b7af53c2cd1e1a505691573f2ba52641f23ca8cbf4814aa3526" {
		t.Log("Checksum doesn't match. Download was faulty.")
		t.FailNow()
	}

	// Try downloading a non-existent file
	err = workspace.DownloadFile(
		"https://github.com/rajch/soedit/releases/download/v1.0/soedit-1.0.tar.gz-notthere",
		dfilename,
	)
	if err == nil {
		t.Log("Trying to download a non-existent file should have returned an error.")
		t.FailNow()
	}

	t.Logf("Download attempt returned error: %v", err)

	// Download file again to check overwriting
	err = workspace.DownloadFile(
		"https://github.com/rajch/soedit/releases/download/v1.0/soedit-1.0.tar.gz",
		dfilename,
	)
	if err != nil {
		t.Logf("Second download failed with error: %v", err)
		t.Fail()
	}
}

func TestRunWithResults(t *testing.T) {
	// Uncomment to see detailed logs
	//kuttilog.Setloglevel(4)

	t.Log("Testing runwithresults with 'hostname'...")
	output, err := workspace.Runwithresults("hostname")
	if err != nil {
		t.Logf("Exec failed with error:%v\n", err)
		t.Fail()
	}
	t.Logf("Output was: \n'%v'\n", output)

	t.Log("Testing runwithresults with 'hostname -i'...")
	output, err = workspace.Runwithresults("hostname", "-i")
	if err != nil {
		t.Logf("Exec failed with error:%v\n", err)
		t.Fail()
	}
	t.Logf("Output was: \n'%v'\n", output)
}
