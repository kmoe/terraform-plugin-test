package tftest

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

const subprocessCurrentSigil = "4acd63807899403ca4859f5bb948d2c6"
const subprocessPreviousSigil = "2279afb8cf71423996be1fd65d32f13b"

// AutoInitProviderHelper is the main entrypoint for testing provider plugins
// using this package. It is intended to be called during TestMain to prepare
// for provider testing.
//
// AutoInitProviderHelper will discover the location of a current Terraform CLI
// executable to test against, detect whether a prior version of the plugin is
// available for upgrade tests, and then will return an object containing the
// results of that initialization which can then be stored in a global variable
// for use in other tests.
func AutoInitProviderHelper(name string) *Helper {
	helper, err := AutoInitHelper("terraform-provider-" + name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot run Terraform provider tests: %s\n", err)
		os.Exit(1)
	}
	return helper
}

// Helper is intended as a per-package singleton created in TestMain which
// other tests in a package can use to create Terraform execution contexts
type Helper struct {
	baseDir                      string
	pluginName                   string
	terraformExec                string
	thisPluginDir, prevPluginDir string
}

// AutoInitHelper uses the auto-discovery behavior of DiscoverConfig to prepare
// a configuration and then calls InitHelper with it. This is a convenient
// way to get the standard init behavior based on environment variables, and
// callers should use this unless they have an unusual requirement that calls
// for constructing a config in a different way.
func AutoInitHelper(pluginName string) (*Helper, error) {
	config, err := DiscoverConfig(pluginName)
	if err != nil {
		return nil, err
	}

	return InitHelper(config)
}

// InitHelper prepares a testing helper with the given configuration.
//
// For most callers it is sufficient to call AutoInitHelper instead, which
// will construct a configuration automatically based on certain environment
// variables.
//
// If this function returns an error then it may have left some temporary files
// behind in the system's temporary directory. There is currently no way to
// automatically clean those up.
func InitHelper(config *Config) (*Helper, error) {
	baseDir, err := ioutil.TempDir("", "tftest-"+config.PluginName)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory for test helper: %s", err)
	}

	var thisPluginDir, prevPluginDir string
	if config.CurrentPluginExec != "" {
		thisPluginDir, err = ioutil.TempDir(baseDir, "plugins-current")
		if err != nil {
			return nil, fmt.Errorf("failed to create temporary directory for -plugin-dir: %s", err)
		}
		currentExecPath := filepath.Join(thisPluginDir, config.PluginName)
		err = os.Symlink(config.CurrentPluginExec, currentExecPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create symlink at %s to %s: %s", currentExecPath, config.CurrentPluginExec, err)
		}
	} else {
		return nil, fmt.Errorf("CurrentPluginExec is not set")
	}
	if config.PreviousPluginExec != "" {
		prevPluginDir, err = ioutil.TempDir(baseDir, "plugins-previous")
		if err != nil {
			return nil, fmt.Errorf("failed to create temporary directory for previous -plugin-dir: %s", err)
		}
		prevExecPath := filepath.Join(prevPluginDir, config.PluginName)
		err = os.Symlink(config.PreviousPluginExec, prevExecPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create symlink at %s to %s: %s", prevExecPath, config.PreviousPluginExec, err)
		}
	}

	return &Helper{
		baseDir:       baseDir,
		pluginName:    config.PluginName,
		terraformExec: config.TerraformExec,
		thisPluginDir: thisPluginDir,
		prevPluginDir: prevPluginDir,
	}, nil
}

// Close cleans up temporary files and directories created to support this
// helper, returning an error if any of the cleanup fails.
//
// Call this before returning from TestMain to minimize the amount of detritus
// left behind in the filesystem after the tests complete.
func (h *Helper) Close() error {
	return os.RemoveAll(h.baseDir)
}

// NewWorkingDir creates a new working directory for use in the implementation
// of a single test, returning a WorkingDir object representing that directory.
//
// If the working directory object is not itself closed by the time the test
// program exits, the Close method on the helper itself will attempt to
// delete it.
func (h *Helper) NewWorkingDir() (*WorkingDir, error) {
	dir, err := ioutil.TempDir(h.baseDir, "work")
	if err != nil {
		return nil, err
	}

	return &WorkingDir{
		h:       h,
		baseDir: dir,
	}, nil
}

// RequireNewWorkingDir is a variant of NewWorkingDir that takes a TestControl
// object and will immediately fail the running test if the creation of the
// working directory fails.
func (h *Helper) RequireNewWorkingDir(t TestControl) *WorkingDir {
	t.Helper()

	wd, err := h.NewWorkingDir()
	if err != nil {
		t := testingT{t}
		t.Fatalf("failed to create new working directory: %s", err)
		return nil
	}
	return wd
}

// HasPreviousVersion returns true if and only if the receiving helper has a
// previous plugin version available for use in tests.
func (h *Helper) HasPreviousVersion() bool {
	return h.prevPluginDir != ""
}

// TerraformExecPath returns the location of the Terraform CLI executable that
// should be used when running tests.
func (h *Helper) TerraformExecPath() string {
	return h.terraformExec
}

// PluginDir returns the directory that should be used as the -plugin-dir when
// running "terraform init" in order to make Terraform detect the current
// version of the plugin.
func (h *Helper) PluginDir() string {
	return h.thisPluginDir
}

// PreviousPluginDir returns the directory that should be used as the -plugin-dir
// when running "terraform init" in order to make Terraform detect the previous
// version of the plugin, if available.
//
// If no previous version is available, this method will panic. Use
// RequirePreviousVersion or HasPreviousVersion to ensure a previous version is
// available before calling this.
func (h *Helper) PreviousPluginDir() string {
	if h.prevPluginDir != "" {
		panic("PreviousPluginDir not available")
	}
	return h.prevPluginDir
}
