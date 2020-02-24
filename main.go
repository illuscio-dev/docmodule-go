package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// exists returns whether the given file or directory exists
func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil { return true, nil }
	if os.IsNotExist(err) { return false, nil }
	return true, err
}

func killDeferred(process *os.Process, shutdownComplete *sync.WaitGroup) {
	defer shutdownComplete.Done()
	log.Println("shutting down godoc server.")
	err := process.Kill()
	if err != nil {
		log.Panicf("error killing godoc server process: %v", err)
	}
	log.Println("go doc server shut down.")
}

func runDocServer(
	settings *Settings,
	shutdownSignal *sync.WaitGroup,
	shutdownComplete *sync.WaitGroup,
) {
	log.Println("starting up godoc server at", settings.ServerHost+".")
	command := exec.Command("godoc", "-http="+settings.ServerHost)

	if err := command.Start(); err != nil {
		log.Panicf("error starting godoc server: %v", err)
	}
	defer killDeferred(command.Process, shutdownComplete)
	shutdownSignal.Wait()
}

func scrapeModulePages(settings *Settings) {
	pathRegex := regexp.QuoteMeta("/pkg/" + settings.ModName) + `|\.css|\.png|\.js`

	wgetCommand := exec.Command(
		"wget",
		// save HTML/CSS documents with proper extensions
		"-E",
		// Convert links to local files
		"-k",
		// get all images, etc. needed to display HTML page
		"-p",
		// don't create directories
		"-nd",
		// specify recursive download
		"-r",
		// No maximum recursion depth
		"-l", "50",
		// don't ascend to the parent directory
		"-np",
		// accept regex
		"--accept-regex", pathRegex,
		// destination directory
		"-P", settings.BuildDir,
		// execute a `.wgetrc'-style command
		"-erobots=off",
		// root path to start crawl
		settings.ServerHost+"/pkg/"+settings.ModName,
	)
	log.Println("wget command:", wgetCommand.Args)
	output, err := wgetCommand.CombinedOutput()

	if err != nil {
		// check if the download worked at all
		exists, err := fileExists(settings.BuildDir + "/style.css")
		if !exists || err != nil {
			log.Panicf(
				"error scraping docs: %v, output: %v", err, string(output),
			)
		}
	}

	log.Print(
		"\n\n##### WGET OUTPUT #####\n\n",
		string(output),
		"\n\n##### END OUTPUT #####\n\n",
	)
}

func waitForServer(settings *Settings) {
	timer := time.AfterFunc(10*time.Second, func() {
		log.Panicf("timeout checking server.")
	})

	client := http.Client{Timeout: 1 * time.Second}
	for true {

		getPath := "http://" + settings.ServerHost + "/pkg/"
		log.Println("Checking Server Status:", getPath)

		resp, err := client.Get(getPath)

		var responsePrint string
		if err != nil {
			responsePrint = err.Error()
		} else {
			responsePrint = resp.Status
		}
		log.Println("Response:", responsePrint)

		if err != nil || resp == nil || resp.StatusCode != 200 {
			time.Sleep(time.Second)
			continue
		} else {
			timer.Stop()
			break
		}
	}
}

func runServerAndScrapeDocs(settings *Settings) {

	// We need to kill godoc if it is running.
	_ = exec.Command("killall", "godoc").Run()

	// Set up a shutdown event to signal to the goroutine running our docs server to
	// kill that process.
	shutDownSignal := sync.WaitGroup{}
	shutDownSignal.Add(1)
	shutDownComplete := sync.WaitGroup{}
	shutDownComplete.Add(1)
	// Defer sending a signal to shutdown the server and wait for it to shut down.
	defer func() {
		shutDownSignal.Done()
		shutDownComplete.Wait()
	}()

	// Run the godoc server in a different goroutine.
	go runDocServer(settings, &shutDownSignal, &shutDownComplete)
	waitForServer(settings)

	// Scrape all the documentation from the server.
	scrapeModulePages(settings)
}

// Making the directory with os.MkDirAll can cause permissions errors that don't occur
// when making each directory individually.
func createBuildDir(path string) {
	// current directory we want to make.
	var makeDir string

	// go through the directories individually.
	for _, subdir := range strings.Split(path, "/") {
		makeDir += "/" + subdir
		makeDir = strings.TrimPrefix(makeDir, "/")
		if err := os.Mkdir(makeDir, os.ModePerm); err != nil {
			if strings.HasSuffix(err.Error(), "file exists") {
				continue
			}
			log.Panicf("error creating build dir: %v", err)
		}
	}
}

// initialize the build directory
func setupBuildDir(settings *Settings) {
	// Clear the build directory.
	if err := os.RemoveAll(settings.BuildDir); err != nil {
		log.Panicf("error removing build directory: %v", err)
	}

	createBuildDir(settings.BuildDir)

	// We want to create a dummy index.html so that when we use wget, that name is
	// reserved for our root file. We can't specify an output file when crawling so we
	// need to reserve it.
	if _, err := os.Create(settings.BuildDir + "/index.html"); err != nil {
		log.Panicf("could not create dummy index: %v", err)
	}
}

func renameEntryPoint(runInfo *RunInfo) (newPath string) {
	settings := runInfo.Settings

	stringSplit := strings.Split(settings.ModName, "/")
	goDocBaseName := stringSplit[len(stringSplit)-1]

	oldPath := settings.BuildDir + "/" + goDocBaseName + ".html"
	newPath = settings.BuildDir + "/" + settings.HTMLBaseName + "-root.html"

	err := os.Rename(oldPath, newPath)
	if err != nil {
		log.Panic("error while renaming entry file:", err)
	}

	runInfo.HtmlFiles = append(runInfo.HtmlFiles, newPath)

	return newPath
}

func renameOutputFiles(runInfo *RunInfo) {
	// make a mapping of the current files to what we want to rename them to.
	settings := runInfo.Settings
	entryPoint := renameEntryPoint(runInfo)

	matches, err := filepath.Glob(settings.BuildDir + "/" + "*" + ".html")
	if err != nil {
		log.Panic("could not find result files:", err.Error())
	}

	for i, oldPath := range matches {
		// skip the entry-point since we have already renamed it
		if oldPath == entryPoint {
			continue
		}
		index := i + 1

		newPath := settings.HTMLBaseName + "." + strconv.Itoa(index) + ".html"
		newPath = settings.BuildDir + "/" + newPath

		err := os.Rename(oldPath, newPath)
		if err != nil {
			log.Panicf("error renaming %q to %q", oldPath, newPath)
		}

		runInfo.DocFileInfo = append(
			runInfo.DocFileInfo, NewDocFileInfo(oldPath, newPath),
		)
		runInfo.HtmlFiles = append(runInfo.HtmlFiles, newPath)
	}
}

// rewrites the internal links of the html files
func rewriteHTMLLinks(runInfo *RunInfo) {

	for _, filePath := range runInfo.HtmlFiles {

		for _, info := range runInfo.DocFileInfo {

			data, err := ioutil.ReadFile(filePath)
			if err != nil {
				log.Panicf("error opening file '%v': %v", filePath, err)
			}

			data = info.HtmlReplaceRegex1.ReplaceAll(data, info.HtmlReplaceWith1)
			data = info.HtmlReplaceRegex2.ReplaceAll(data, info.HtmlReplaceWith2)

			err = ioutil.WriteFile(filePath, data, os.ModePerm)
			if err != nil {
				log.Panicf("error altering output file: %v", err)
			}
		}
	}

}

func main() {
	runInfo := setupRunInfo()
	setupBuildDir(runInfo.Settings)
	runServerAndScrapeDocs(runInfo.Settings)
	renameOutputFiles(runInfo)
	rewriteHTMLLinks(runInfo)
}
