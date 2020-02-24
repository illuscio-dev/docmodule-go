package main

import (
	"encoding/json"
	"flag"
	"golang.org/x/xerrors"
	"io/ioutil"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
)

type RunInfo struct {
	Settings    *Settings
	HtmlFiles   []string
	DocFileInfo []*DocFileInfo
}

// Call to initialize a blank object without nil pointers.
func NewRunInfo() *RunInfo {
	return &RunInfo{
		Settings:    new(Settings),
		DocFileInfo: make([]*DocFileInfo, 0),
	}
}

type DocFileInfo struct {
	OldName           string
	NewName           string
	HtmlReplaceRegex1 *regexp.Regexp
	HtmlReplaceRegex2 *regexp.Regexp
	HtmlReplaceWith1  []byte
	HtmlReplaceWith2  []byte
}

func NewDocFileInfo(oldPath string, newPath string) *DocFileInfo {
	oldName := filepath.Base(oldPath)
	newName := filepath.Base(newPath)

	regex1, _ := regexp.Compile("href=\"" + oldName + "#")
	regex2, _ := regexp.Compile("href=\"" + oldName + "\"")

	docFileInfo := DocFileInfo{
		OldName:           oldName,
		NewName:           newName,
		HtmlReplaceRegex1: regex1,
		HtmlReplaceRegex2: regex2,
		HtmlReplaceWith1:  []byte("href=\"" + newName + "#"),
		HtmlReplaceWith2:  []byte("href=\"" + newName + "\""),
	}

	return &docFileInfo
}

type CliArgs struct {
	// GoDoc server host
	ServerHost *string
	// Build Directory
	BuildDir *string
	// Base name to use for html files
	HTMLBaseName *string
}

type Settings struct {
	// $GOROOT value
	GoRootPath string `json:"GOROOT"`
	// $GOPATH value
	GoPath string `json:"GOPATH"`
	// Path to go.mod
	GoModPath string `json:"GOMOD"`
	// Module name
	ModName string
	// Path to root of module
	ModuleRootPath string
	// GoDoc server host
	ServerHost string
	// Build Directory
	BuildDir string
	// Base name to use for html files
	HTMLBaseName string
}

// Path to root module page on godoc server.
func (settings *Settings) serverModulePath() string {
	return settings.ServerHost + "/" + settings.ModName
}

// Regex for extracting module name from go.mod file
var modNameRegex = regexp.MustCompile(`module\s+(?P<modName>\S+)`)

// Extracts information we are interested in via the go env command
func getEnvSettings(settings *Settings) {
	// Run the command
	envJsonBytes, err := exec.Command("go", "env", "-json").Output()
	if err != nil {
		log.Fatal(xerrors.Errorf("error inspecting go environment: %w", err))
	}

	// Marshal the settings we want to our struct.
	err = json.Unmarshal(envJsonBytes, settings)
	if err != nil {
		log.Fatal(xerrors.Errorf("error parsing go environment: %w", err))
	}

	settings.ModuleRootPath = filepath.Dir(settings.GoModPath)

	settings.ServerHost = "localhost:6161"
}

func applyCliArgs(settings *Settings, args *CliArgs) {

	settings.BuildDir = *args.BuildDir
	settings.ServerHost = *args.ServerHost
	settings.HTMLBaseName = *args.HTMLBaseName
}

// Gets the package name from go mod
func getGoModName(settings *Settings) {
	goModContent, err := ioutil.ReadFile(settings.GoModPath)
	if err != nil {
		log.Fatal("error reading go mod")
	}

	match := modNameRegex.FindSubmatch(goModContent)
	// throw an error if we couldn't parse the module name
	if len(match) < 2 {
		log.Fatal("could not find module name in go.mod")
	}

	settings.ModName = string(match[1])
}

func parseCmdArgs() *CliArgs {
	cliArgs := new(CliArgs)
	cliArgs.BuildDir = flag.String(
		"--build-path",
		"zdocs/source/_static",

		"path to place extracted html files",
	)
	cliArgs.ServerHost = flag.String(
		"--godoc-host",
		"localhost:6161",
		"Host and port to use for temporarily running godoc server.",
	)
	cliArgs.HTMLBaseName = flag.String(
		"--html-file-name",
		"godoc",
		"Base name to use for extracted html files.",
	)

	flag.Parse()

	return cliArgs
}

func setupRunInfo() *RunInfo {
	cliArgs := parseCmdArgs()
	runInfo := NewRunInfo()
	getEnvSettings(runInfo.Settings)
	getGoModName(runInfo.Settings)
	applyCliArgs(runInfo.Settings, cliArgs)
	return runInfo
}
