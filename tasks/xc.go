package tasks

/*
   Copyright 2013 Am Laher

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

import (
	"errors"
	//Tip for Forkers: please 'clone' from my url and then 'pull' from your url. That way you wont need to change the import path.
	//see https://groups.google.com/forum/?fromgroups=#!starred/golang-nuts/CY7o2aVNGZY
	"github.com/openxo/goxc/archive/ar"
	"github.com/openxo/goxc/config"
	"github.com/openxo/goxc/core"
	"github.com/openxo/goxc/executils"
	"github.com/openxo/goxc/exefileparse"
	"github.com/openxo/goxc/platforms"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//runs automatically
func init() {
	//GOARM=6 (this is the default for go1.1
	Register(Task{
		"xc",
		"Cross compile. Builds executables for other platforms.",
		runTaskXC,
		map[string]interface{}{"GOARM": "",
			//"validation" : "tcBinExists,exeParse",
			"validateToolchain":    true,
			"verifyExe":            true,
			"autoRebuildToolchain": true}})
}

func runTaskXC(tp TaskParams) error {
	if len(tp.DestPlatforms) == 0 {
		return errors.New("No valid platforms specified")
	}
	success := 0
	var err error
	appName := core.GetAppName(tp.WorkingDirectory)
	outDestRoot := core.GetOutDestRoot(appName, tp.Settings.ArtifactsDest, tp.WorkingDirectory)
	log.Printf("mainDirs : %v", tp.MainDirs)
	for _, dest := range tp.DestPlatforms {
		for _, mainDir := range tp.MainDirs {
			exeName := filepath.Base(mainDir)
			absoluteBin, err := xcPlat(dest.Os, dest.Arch, mainDir, tp.Settings, outDestRoot, exeName)
			if err != nil {
				log.Printf("Error: %v", err)
				log.Printf("Have you run `goxc -t` for this platform (%s,%s)???", dest.Arch, dest.Os)
				return err
			} else {
				success = success + 1
				isVerifyExe := tp.Settings.GetTaskSettingBool(TASK_XC, "verifyExe")
				if isVerifyExe {
					err = exefileparse.Test(absoluteBin, dest.Arch, dest.Os)
					if err != nil {
						log.Printf("Error: %v", err)
						log.Printf("Something fishy is going on: have you run `goxc -t` for this platform (%s,%s)???", dest.Arch, dest.Os)
						return err
					}
				}
			}
		}
	}
	//0.6 return error if no platforms succeeded.
	if success < 1 {
		log.Printf("No successes!")
		return err
	}
	return nil
}

func validateToolchain(goos, arch, goroot string) error {
	err := validatePlatToolchainBinExists(goos, arch, goroot)
	if err != nil {
		return err
	}
	err = validatePlatToolchainPackageVersion(goos, arch, goroot)
	if err != nil {
		return err
	}

	return nil
}

func validatePlatToolchainPackageVersion(goos, arch, goroot string) error {
	platPkgFileRuntime := filepath.Join(goroot, "pkg", goos+"_"+arch, "runtime.a")
	nr, err := os.Open(platPkgFileRuntime)
	if err != nil {
		log.Printf("Could not validate toolchain version: %v", err)
	}
	tr, err := ar.NewReader(nr)
	if err != nil {
		log.Printf("Could not validate toolchain version: %v", err)
	}
	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				log.Printf("Could not validate toolchain version: %v", err)
				return nil
			}
			log.Printf("Could not validate toolchain version: %v", err)
			return err
		}
		//log.Printf("Header: %+v", h)
		if h.Name == "__.PKGDEF" {
			firstLine, err := tr.NextString(50)
			if err != nil {
				log.Printf("failed to read first line of PKGDEF: %v", err)
				return nil
			}
			//log.Printf("pkgdef first part: '%s'", firstLine)
			expectedPrefix := "go object " + goos + " " + arch + " "
			if !strings.HasPrefix(firstLine, expectedPrefix) {
				log.Printf("first line of __.PKGDEF does not match expected pattern: %v", expectedPrefix)
				return nil
			}
			parts := strings.Split(firstLine, " ")
			compiledVersion := parts[4]
			//runtimeVersion := runtime.Version()
			//log.Printf("Runtime version: %s", runtimeVersion)
			cmdPath := filepath.Join(goroot, "bin", "go")
			cmd := exec.Command(cmdPath)
			args := []string{"version"}
			err = executils.PrepareCmd(cmd, ".", args, []string{}, false)
			if err != nil {
				log.Printf("`go version` failed: %v", err)
				return nil
			}
			goVersionOutput, err := cmd.Output()
			if err != nil {
				log.Printf("`go version` failed: %v", err)
				return nil
			}
			//log.Printf("output: %s", string(out))
			goVersionOutputParts := strings.Split(string(goVersionOutput), " ")
			goVersion := goVersionOutputParts[2]
			if compiledVersion != goVersion {
				return errors.New("static library version '" + compiledVersion + "' does NOT match `go version` '" + goVersion + "'!")
			}
			log.Printf("Toolchain version '%s' verified against 'go' executable version '%s'", compiledVersion, goVersion)
			return nil
		}
	}
}

func validatePlatToolchainBinExists(goos, arch, goroot string) error {
	platGoBin := filepath.Join(goroot, "bin", goos+"_"+arch, "go")
	if goos == runtime.GOOS && arch == runtime.GOARCH {

		platGoBin = filepath.Join(goroot, "bin", "go")
	}
	if goos == platforms.WINDOWS {
		platGoBin += ".exe"
	}
	_, err := os.Stat(platGoBin)
	return err
}

// xcPlat: Cross compile for a particular platform
// 0.3.0 - breaking change - changed 'call []string' to 'workingDirectory string'.
func xcPlat(goos, arch string, workingDirectory string, settings config.Settings, outDestRoot string, exeName string) (string, error) {
	isValidateToolchain := settings.GetTaskSettingBool(TASK_XC, "validateToolchain")
	goroot := settings.GoRoot
	if isValidateToolchain {
		err := validateToolchain(goos, arch, goroot)
		if err != nil {
			log.Printf("Toolchain not ready. Re-building toolchain. (%v)", err)
			isAutoToolchain := settings.GetTaskSettingBool(TASK_XC, "autoRebuildToolchain")
			if isAutoToolchain {
				err = buildToolchain(goos, arch, settings)
			}
			if err != nil {
				return "", err
			}
		}
	}
	log.Printf("building %s for platform %s_%s.", exeName, goos, arch)
	relativeDir := filepath.Join(settings.GetFullVersionName(), goos+"_"+arch)

	outDir := filepath.Join(outDestRoot, relativeDir)
	err := os.MkdirAll(outDir, 0755)
	if err != nil {
		return "", err
	}
	args := []string{}
	relativeBin := core.GetRelativeBin(goos, arch, exeName, false, settings.GetFullVersionName())
	absoluteBin := filepath.Join(outDestRoot, relativeBin)
	//args = append(args, executils.GetLdFlagVersionArgs(settings.GetFullVersionName())...)
	args = append(args, "-o", absoluteBin, ".")
	//log.Printf("building %s", exeName)
	//v0.8.5 no longer using CGO_ENABLED
	envExtra := []string{"GOOS=" + goos, "GOARCH=" + arch}
	if goos == platforms.LINUX && arch == platforms.ARM {
		// see http://dave.cheney.net/2012/09/08/an-introduction-to-cross-compilation-with-go
		goarm := settings.GetTaskSettingString(TASK_XC, "GOARM")
		if goarm != "" {
			envExtra = append(envExtra, "GOARM="+goarm)
		}
	}
	err = executils.InvokeGo(workingDirectory, "build", args, envExtra, settings)
	return absoluteBin, err
}
