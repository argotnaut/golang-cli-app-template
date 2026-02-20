package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const APP = "gcat"

// This line is updated automatically by ../.github/workflows/release.yaml
const VERSION = "1.0.0"

func moveBinary(destination string, app string) string {
	executablePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Error getting executable path: %+v\n", err)
	}
	executablePath, err = filepath.EvalSymlinks(executablePath)
	if err != nil {
		log.Fatalf("Error getting executable path: %+v\n", err)
	}
	exeBytes, err := os.ReadFile(executablePath)
	if err != nil {
		log.Fatalf("Error reading the current binary file: %+v\n", err)
	}
	outputPath := filepath.Join(destination, app)
	err = os.WriteFile(outputPath, exeBytes, 0644)
	if err != nil {
		log.Fatalf("Error writing the binary file: %+v\n", err)
	}
	err = os.Chmod(outputPath, 0755)
	if err != nil {
		log.Fatalf("Error making the binary file executable: %+v\n", err)
	}
	return outputPath
}

func stringExistsInFile(path string, str string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), str) {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func installWindowsBinary(rootCmd cobra.Command) {
	const EXE_NAME = APP + ".exe"
	const APP_DIR = "." + APP
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Error getting home directory: %v\n", err)
	}
	getPSHOMECmd := exec.Command("powershell", "-command", "\"$PSHOME\"")
	getPSHOME, err := getPSHOMECmd.Output()
	if _, ok := err.(*exec.ExitError); err != nil && !ok {
		log.Fatalf("Error getting $PSHOME: %+v", err)
	}
	PSHome := filepath.Clean(strings.TrimSpace(string(getPSHOME)))
	// Create app directory
	err = os.MkdirAll(filepath.Join(home, APP_DIR), os.ModePerm)
	if err != nil {
		log.Fatalf("Error creating %s\n", APP_DIR)
	}

	moveBinary(filepath.Join(home, APP_DIR), EXE_NAME)
	// Generate completions
	completionsCodeBuffer := bytes.NewBufferString("")
	rootCmd.GenPowerShellCompletionWithDesc(completionsCodeBuffer)
	// Write completions to app-specific completions script
	completionsFilePath := filepath.Join(home, APP_DIR, APP+"Completions.ps1")
	completionsFile, err := os.OpenFile(
		completionsFilePath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644,
	)
	if err != nil {
		log.Fatalf("Error opening completions file %s: %+v", completionsFilePath, err)
	}
	defer completionsFile.Close()
	_, err = completionsFile.Write(completionsCodeBuffer.Bytes())
	if err != nil {
		log.Fatalf("Error writing to completions file %s: %+v", completionsFilePath, err)
	}
	// Ensure the $PSHOME\Profile.ps1 script exists
	psProfile := filepath.Join(PSHome, "Profile.ps1")
	profileFile, err := os.OpenFile(
		filepath.Join(psProfile),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644,
	)
	if err != nil {
		log.Fatalf("Error opening completions file %s: %+v", psProfile, err)
	}
	profileFile.Close()
	// Add app-specific completions script to main profile (if it's not already there)
	found, err := stringExistsInFile(psProfile, completionsFilePath)
	if err != nil {
		log.Fatalf("Error writing app-specific completions script to profile: %+v\n", err)
	}
	if !found {
		// Write a line in the profile script that will call the completions script every time powershell opens
		profileFile, err := os.OpenFile(
			filepath.Join(psProfile),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0644,
		)
		if err != nil {
			log.Fatalf("Error opening completions file %s: %+v", psProfile, err)
		}
		defer profileFile.Close()
		_, err = profileFile.Write([]byte(". " + completionsFilePath))
		if err != nil {
			log.Fatalf("Error writing to completions file %s: %+v", psProfile, err)
		}
	}
	// Set completions
	setExecPolicyCmd := exec.Command(
		"powershell",
		"-command",
		"\"Set-ExecutionPolicy -Scope CurrentUser unrestricted\"",
	)
	_, err = setExecPolicyCmd.Output()
	if _, ok := err.(*exec.ExitError); err != nil && !ok {
		log.Fatalf("Error setting execution policy: %+v", err)
	}
	setCompletionsCmd := exec.Command(
		"powershell",
		"-command",
		"\"Set-PSReadlineKeyHandler -Key Tab -Function Complete\"",
	)
	_, err = setCompletionsCmd.Output()
	if _, ok := err.(*exec.ExitError); err != nil && !ok {
		log.Fatalf("Error setting completions: %+v", err)
	}
	// Update path
	updatePathCmd := exec.Command(
		"powershell",
		"-command",
		`$env:Path += ";$FULL_APP_DIR_PATH";
		"Adding app to path";
		$registryItem = 'Registry::HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Session Manager\Environment';
		$oldPath = (Get-Item $registryItem).GetValue('Path', $null, 'DoNotExpandEnvironmentNames');
		$newPath = "$FULL_APP_DIR_PATH;$oldPath";`,
	)
	_, err = updatePathCmd.Output()
	if _, ok := err.(*exec.ExitError); err != nil && !ok {
		log.Fatalf("Error updating path: %+v", err)
	}
}

func uninstallWindowsBinary() {
	uninstallCmd := exec.Command(
		"powershell",
		"-command",
		`get-content $PSHOME\Profile.ps1 | % {$_.Replace(". $env:USERPROFILE\.`+APP+`\`+APP+`Completions.ps1","")} | Out-File $PSHOME\Profile.ps1; # remove completions
		rm -Recurse -Force $env:USERPROFILE\.`+APP+`; # remove app directory`,
	)
	_, err := uninstallCmd.Output()
	if _, ok := err.(*exec.ExitError); err != nil && !ok {
		log.Fatalf("Error uninstalling: %+v", err)
	}
}

const COMPLETIONS_DIR = "/usr/share/bash-completion/completions/"
const LINK_DIRECTORY = "/usr/local/bin/"
const SHELL = "bash"
const SYMLINK = LINK_DIRECTORY + APP

func installLinuxBinary(rootCmd cobra.Command) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Error getting home directory: %v\n", err)
	}
	appDir := filepath.Join(home, "bin", APP)
	configDir := filepath.Join(home, "."+APP)

	err = os.MkdirAll(appDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Error creating %s\n", appDir)
	}
	err = os.MkdirAll(configDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Error creating %s\n", appDir)
	}

	zshCmd := exec.Command("which", "zsh")
	zsh, err := zshCmd.Output()
	if _, ok := err.(*exec.ExitError); !ok {
		log.Fatalf("Error checking for zsh: %+v", err)
	}

	if len(zsh) == 0 {
		const BASH_COMPLETIONS_FILE = COMPLETIONS_DIR + APP
		// Generate completions
		completionsCodeBuffer := bytes.NewBufferString("")
		rootCmd.GenBashCompletionV2(completionsCodeBuffer, true)
		completionsCode := completionsCodeBuffer.String()
		// Ensure the completions file gets created correctly
		createCompletionsFile := exec.Command(
			SHELL,
			"-c",
			"USER=$(whoami);sudo touch "+BASH_COMPLETIONS_FILE+"; sudo chown -R $USER "+BASH_COMPLETIONS_FILE,
		)
		_, err = createCompletionsFile.Output()
		if _, ok := err.(*exec.ExitError); err != nil && !ok {
			log.Fatalf("Error creating completions file: %+v", err)
		}
		// Write completions
		completionsFile, err := os.OpenFile(
			filepath.Join(BASH_COMPLETIONS_FILE),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0644,
		)
		if err != nil {
			log.Fatalf("Error opening completions file %s: %+v", BASH_COMPLETIONS_FILE, err)
		}
		defer completionsFile.Close()
		_, err = completionsFile.Write([]byte(completionsCode))
		if err != nil {
			log.Fatalf("Error writing to completions file %s: %+v", BASH_COMPLETIONS_FILE, err)
		}
		// Update completions in current shell
		_, err = exec.Command("/bin/sh", filepath.Join(home, BASH_COMPLETIONS_FILE)).Output()
		if _, ok := err.(*exec.ExitError); !ok {
			log.Fatalf("Error running %s: %+v", BASH_COMPLETIONS_FILE, err)
		}
	}

	// Move the binary file to the correct directory
	newPath := moveBinary(appDir, APP)
	// Make a symlink to the binary file in a PATH directory
	err = os.MkdirAll(LINK_DIRECTORY, 0755)
	if err != nil {
		log.Fatalf("Error creating %s: %+v\n", LINK_DIRECTORY, err)
	}

	// (This is done through bash because it requires elevation)
	createSymLink := exec.Command(
		SHELL,
		"-c",
		"sudo ln -n "+newPath+" "+SYMLINK,
	)
	_, err = createSymLink.Output()
	if _, ok := err.(*exec.ExitError); err != nil && !ok {
		log.Fatalf("Error creating symlink: %+v", err)
	}
}

func uninstallLinuxBinary() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Error getting home directory: %v\n", err)
	}
	appDir := filepath.Join(home, "bin", APP)
	configDir := filepath.Join(home, "."+APP)
	completionsDir := COMPLETIONS_DIR + APP

	filesToDelete := []string{appDir, configDir, completionsDir, SYMLINK}
	for _, file := range filesToDelete {
		deleteCmd := exec.Command(
			SHELL,
			"-c",
			"sudo rm -rf "+file,
		)
		_, err := deleteCmd.Output()
		if err != nil {
			log.Fatalf("Error deleting %s: %e\n", file, err)
		}
		fmt.Printf("Deleted %s\n", file)
	}
}

var rootCmd = &cobra.Command{
	Use:   APP,
}

func init() {

	rootCmd.Flags().BoolP("version", "v", false, "Prints the current version")
	rootCmd.Flags().Bool("install", false, "Installs this binary on the current system")
	rootCmd.Flags().Bool("uninstall", false, "Uninstalls this binary from the current system")

	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		showVersion, err := cmd.Flags().GetBool("version")
		if err != nil {
			log.Fatalln(err)
		}
		install, err := cmd.Flags().GetBool("install")
		if err != nil {
			log.Fatalln(err)
		}
		uninstall, err := cmd.Flags().GetBool("uninstall")
		if err != nil {
			log.Fatalln(err)
		}

		if install {
			switch runtime.GOOS {
			case "linux":
				installLinuxBinary(*rootCmd)
			case "windows":
				installWindowsBinary(*rootCmd)
			case "darwin":
				installLinuxBinary(*rootCmd)
			default:
				log.Fatalln("unsupported platform")
			}
		}
		if uninstall {
			switch runtime.GOOS {
			case "linux":
				uninstallLinuxBinary()
			case "windows":
				uninstallWindowsBinary()
			case "darwin":
				uninstallLinuxBinary()
			default:
				log.Fatalln("unsupported platform")
			}
			return
		}
		if showVersion {
			fmt.Println(VERSION)
		}
		if !showVersion && !install {
			cmd.Help()
		}
	}
}

func RootCmd() *cobra.Command {
	return rootCmd
}
