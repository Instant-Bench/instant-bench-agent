package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
)

var debugMode bool
var spinnerInstance *spinner.Spinner

// Logger provides methods for printing debug and info messages
func debugLog(format string, args ...interface{}) {
	if debugMode {
		if spinnerInstance != nil && spinnerInstance.Active() {
			spinnerInstance.Stop()
			defer spinnerInstance.Start()
		}
		color.Cyan("[DEBUG] "+format, args...)
	}
}

func infoLog(format string, args ...interface{}) {
	if spinnerInstance != nil && spinnerInstance.Active() {
		spinnerInstance.Stop()
		defer spinnerInstance.Start()
	}
	color.Blue(format, args...)
}

func errorLog(format string, args ...interface{}) {
	if spinnerInstance != nil && spinnerInstance.Active() {
		spinnerInstance.Stop()
	}
	color.Red("❌ "+format, args...)
}

func successLog(format string, args ...interface{}) {
	if spinnerInstance != nil && spinnerInstance.Active() {
		spinnerInstance.Stop()
	}
	color.Green("✓ "+format, args...)
}

func startSpinner(message string) {
	if spinnerInstance != nil {
		spinnerInstance.Stop()
	}
	spinnerInstance = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	spinnerInstance.Suffix = " " + message
	spinnerInstance.Color("blue")
	spinnerInstance.Start()
}

func stopSpinner() {
	if spinnerInstance != nil && spinnerInstance.Active() {
		spinnerInstance.Stop()
	}
}

func filterOutput(output []byte) []byte {
	lines := bytes.Split(output, []byte("\n"))
	var filteredOutput []byte
	captureOutput := false
	for _, line := range lines {

		if bytes.Contains(line, []byte("BENCHMARK_START")) {
			captureOutput = true
			continue
		}

		if bytes.Contains(line, []byte("BENCHMARK_END")) {
			captureOutput = false
			continue
		}

		if bytes.Contains(line, []byte("Still creating")) {
			continue
		}

		if captureOutput == true {
			// Remove "Remote-Output:" prefix if present
			if bytes.Contains(line, []byte("Remote-Output:")) {
				line = bytes.Replace(line, []byte("Remote-Output:"), []byte(""), 1)
			}
			
			filteredOutput = append(filteredOutput, line...)
			filteredOutput = append(filteredOutput, '\n')
		}
	}
	return filteredOutput
}

var terraformPath string

func getTerraformDir() string {
    if terraformPath == "" {
		terraformPath = "../aws"
    }

    // Ensure the path is absolute
    absolutePath, err := filepath.Abs(terraformPath)
    if err != nil {
        log.Fatalf("Failed to resolve absolute path: %v", err)
    }

    if _, err := os.Stat(absolutePath); os.IsNotExist(err) {
        log.Fatalf("Terraform directory does not exist: %s", absolutePath)
    }

    return absolutePath
}

// copyDir recursively copies a directory tree, attempting to preserve permissions.
func copyDir(src, dst string) error {
	// Get properties of source dir
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination dir
	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return err
	}

	directory, err := os.Open(src)
	if err != nil {
		return err
	}
	defer directory.Close()

	entries, err := directory.Readdir(-1)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursive call for directories
			err = copyDir(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			// Copy files
			err = copyFile(srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func main() {
	// Define command line flags
	useExistingMachine := flag.String("host", "", "IP address of an existing machine to run the benchmark on")
	sshKeyPath := flag.String("ssh-key", "", "Path to SSH private key for connecting to existing machine")
	sshUser := flag.String("ssh-user", "ubuntu", "SSH username for connecting to existing machine")
	folderPath := flag.String("folder", "", "Path to folder containing all dependencies to be copied")
	command := flag.String("command", "", "Custom command to run on the instance")
	instanceType := flag.String("instance-type", "t2.micro", "AWS instance type to use")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	debugMode = *debug

	args := flag.Args()
	var binaryPath string
	var cmdToRun string
	var filesToCopy []string
	var binariesToCopy []string
	var cmdParts []string

	if len(args) == 1 {
		// Single positional argument is treated as a command
		cmdToRun = args[0]
		// Try to infer binary from command
		cmdParts = strings.Fields(cmdToRun)
		if len(cmdParts) > 0 {
			inferredBinary, err := exec.LookPath(cmdParts[0])
			if err == nil {
				binaryPath = inferredBinary
				fmt.Printf("Inferred binary from command: %s\n", binaryPath)
			} else {
				fmt.Printf("Warning: Could not find binary '%s' in PATH. Will rely on remote system having it installed.\n", cmdParts[0])
			}
		}
	} else if len(args) > 1 {
		// For backward compatibility, if there are two or more arguments,
		// treat the first as binary and join the rest as a command
		binary := args[0]
		var err error
		binaryPath, err = exec.LookPath(binary)
		if err != nil {
			log.Fatalf("Binary %s not found in PATH", binary)
		}
		
		// Join the remaining arguments as the command
		cmdToRun = strings.Join(args[1:], " ")
	} else if *command != "" {
		// Command flag is provided
		cmdToRun = *command
		// Try to infer binary from command
		cmdParts = strings.Fields(cmdToRun)
		if len(cmdParts) > 0 {
			inferredBinary, err := exec.LookPath(cmdParts[0])
			if err == nil {
				binaryPath = inferredBinary
				fmt.Printf("Inferred binary from command: %s\n", binaryPath)
			} else {
				fmt.Printf("Warning: Could not find binary '%s' in PATH. Will rely on remote system having it installed.\n", cmdParts[0])
			}
		}
	} else {
		printUsageAndExit()
	}
	
	// Check for multiple commands with && and add their binaries
	if strings.Contains(cmdToRun, "&&") {
		commands := strings.Split(cmdToRun, "&&")
		debugLog("Found %d commands in compound statement", len(commands))
		
		for i, cmd := range commands {
			cmd = strings.TrimSpace(cmd)
			if cmd == "" {
				continue
			}
			
			cmdParts = strings.Fields(cmd)
			if len(cmdParts) > 0 {
				binCmd := cmdParts[0]
				inferredBinary, err := exec.LookPath(binCmd)
				if err == nil {
					debugLog("Inferred additional binary from command part %d: %s", i+1, inferredBinary)
					if !contains(binariesToCopy, inferredBinary) {
						binariesToCopy = append(binariesToCopy, inferredBinary)
					}
				} else {
					debugLog("Could not find binary '%s' in PATH for command part %d", binCmd, i+1)
				}
			}
		}
	}
	
	// Find files to copy
	cmdParts = strings.Fields(cmdToRun)
	for i, part := range cmdParts {
		// Skip the binary and any flags
		if strings.HasPrefix(part, "-") || (i == 0) {
			continue
		}
		
		// Remove any quotes
		part = strings.Trim(part, "'\"")
		
		// Check if this part looks like a file path
		if (!strings.HasPrefix(part, "-") && fileExists(part)) || (strings.Contains(part, ".") && !strings.HasPrefix(part, "-") && fileExists(part)) {
			// It's a file, add it to the list
			absPath, err := filepath.Abs(part)
			if err == nil {
				if !contains(filesToCopy, absPath) {
					filesToCopy = append(filesToCopy, absPath)
					fmt.Printf("Found file in command: %s\n", absPath)
				}
			}
		}
	}
	
	tmpFolder, err := os.MkdirTemp(".", ".ib-")
	debugLog("Created temporary folder %s", tmpFolder)
	if err != nil {
		log.Fatalf("Failed to create temporary folder: %v", err)
	}
	
	fullTempFolder, err := filepath.Abs(tmpFolder)
	if err != nil {
		log.Fatalf("Failed to get absolute path for temporary folder: %v", err)
	}

	// Copy files to temporary folder
	remappedPaths := make(map[string]string)
	
	// First copy all detected binaries
	for _, binary := range binariesToCopy {
		// Skip if already in filesToCopy
		if contains(filesToCopy, binary) {
			continue
		}
		
		// Get the binary name
		binaryName := filepath.Base(binary)
		
		// Create the destination path in the temp folder
		destPath := filepath.Join(tmpFolder, binaryName)
		
		// Copy the binary
		err = copyFile(binary, destPath)
		if err != nil {
			debugLog("Warning: Failed to copy binary %s: %v", binary, err)
		} else {
			debugLog("Copied binary: %s to %s", binary, destPath)
			remappedPaths[binary] = binaryName
		}
	}
	
	// Now copy individual files identified from the command
	for _, file := range filesToCopy {
		// Get the file's directory
		fileName := filepath.Base(file)
		
		// Create the destination path in the temp folder
		destPath := filepath.Join(tmpFolder, fileName)
		
		// Skip if already copied as a binary
		if _, exists := remappedPaths[file]; exists {
			continue
		}
		
		// Copy the file
		err = copyFile(file, destPath)
		if err != nil {
			debugLog("Warning: Failed to copy file %s: %v", file, err)
		} else {
			debugLog("Copied file: %s to %s", file, destPath)
			remappedPaths[file] = fileName
		}
	}
	
	// Copy folder if specified
	if *folderPath != "" {
		// Convert to absolute path if needed
		absPath := *folderPath
		if !filepath.IsAbs(absPath) {
			cwd, err := os.Getwd()
			if err != nil {
				log.Fatalf("Failed to get working directory: %v", err)
			}
			absPath = filepath.Join(cwd, absPath)
		}
		
		// Verify the folder exists
		info, err := os.Stat(absPath)
		if err != nil {
			log.Fatalf("Failed to access folder %s: %v", absPath, err)
		}
		if !info.IsDir() {
			log.Fatalf("%s is not a directory", absPath)
		}
		
		fmt.Printf("Copying folder %s to benchmark environment...\n", absPath)
		
		// Preserve the folder structure by creating a subfolder with the same name
		folderName := filepath.Base(absPath)
		folderDestPath := filepath.Join(tmpFolder, folderName)
		err = os.MkdirAll(folderDestPath, 0755)
		if err != nil {
			errorLog("Failed to create directory %s: %v", folderDestPath, err)
			os.Exit(1)
		}
		
		startSpinner("Copying " + folderName + " files...")
		err = copyDir(absPath, folderDestPath)
		stopSpinner()
		if err != nil {
			errorLog("Failed to copy folder: %v", err)
			os.Exit(1)
		}
		successLog("Folder %s copied successfully", folderName)
		
		// Adjust the command to use the correct paths in the remote environment
		cmdParts = strings.Fields(cmdToRun)
		if len(cmdParts) > 0 {
			// Keep the binary name the same
			newCmd := []string{cmdParts[0]}
			
			// Adjust paths for other arguments
			for i := 1; i < len(cmdParts); i++ {
				part := cmdParts[i]
				// Remove any quotes
				part = strings.Trim(part, "'\"")
				
				if strings.HasPrefix(part, *folderPath) {
					// If the path starts with the folder path, replace it with the relative path
					relPath, err := filepath.Rel(*folderPath, part)
					if err == nil {
						// Include the folder name in the path to maintain the structure
						newPath := filepath.Join(folderName, relPath)
						newCmd = append(newCmd, newPath)
						continue
					}
				}
				
				// Check if this is a file path we've copied
				absFilePath, err := filepath.Abs(part)
				if err == nil && remappedPaths[absFilePath] != "" {
					// Replace with just the file name
					newCmd = append(newCmd, remappedPaths[absFilePath])
				} else {
					// Keep as is
					newCmd = append(newCmd, part)
				}
			}
			
			cmdToRun = strings.Join(newCmd, " ")
			debugLog("Adjusted command for remote environment: %s", cmdToRun)
		}
	}
	
	// Run on existing machine if specified
	if *useExistingMachine != "" {
		infoLog("Running benchmark on existing machine: %s", *useExistingMachine)
		
		// Validate SSH key if provided
		if *sshKeyPath != "" {
			if _, err := os.Stat(*sshKeyPath); os.IsNotExist(err) {
				log.Fatalf("SSH key file not found: %s", *sshKeyPath)
			}
		}
		
		// Create benchmark directory on remote machine
		var sshKeyOption string
		if *sshKeyPath != "" {
			sshKeyOption = "-i " + *sshKeyPath
		}
		
		createDirCmd := exec.Command("ssh", strings.Split(sshKeyOption+" "+*sshUser+"@"+*useExistingMachine+" mkdir -p /home/"+*sshUser+"/benchmark", " ")...)
		startSpinner("Preparing remote environment...")
		createDirCmd.Stdout = os.Stdout
		createDirCmd.Stderr = os.Stderr
		err = createDirCmd.Run()
		stopSpinner()
		if err != nil {
			errorLog("Failed to create benchmark directory on remote machine: %v", err)
			os.Exit(1)
		}
		
		// Copy files to remote machine
		scpCmd := exec.Command("scp", append(strings.Split(sshKeyOption+" -r ", " "), 
			tmpFolder+"/*", *sshUser+"@"+*useExistingMachine+":/home/"+*sshUser+"/benchmark/")...)
		startSpinner("Copying files to remote machine...")
		scpCmd.Stdout = os.Stdout
		scpCmd.Stderr = os.Stderr
		err = scpCmd.Run()
		stopSpinner()
		if err != nil {
			errorLog("Failed to copy files to remote machine: %v", err)
			os.Exit(1)
		}
		
		// Create a temporary script to run on the remote machine
		scriptPath := filepath.Join(tmpFolder, "run_benchmark.sh")
		scriptContent := `#!/bin/bash
cd /home/` + *sshUser + `/benchmark
echo "BENCHMARK_START"
echo "Run 1"
` + cmdToRun + `
echo "Run 2"
` + cmdToRun + `
echo "Run 3"
` + cmdToRun + `
echo "BENCHMARK_END"
`
		err = os.WriteFile(scriptPath, []byte(scriptContent), 0755)
		if err != nil {
			log.Fatalf("Failed to create benchmark script: %v", err)
		}
		
		// Run benchmark script on remote machine
		infoLog("Running benchmark...")
		sshCmd := exec.Command("ssh", strings.Split(sshKeyOption+" "+*sshUser+"@"+*useExistingMachine+" bash /home/"+*sshUser+"/benchmark/run_benchmark.sh", " ")...)
		output, err := sshCmd.CombinedOutput()
		if err != nil {
			errorLog("Failed to run benchmark on remote machine: %v\nOutput: %s", err, output)
			os.Exit(1)
		}
		
		fmt.Println(string(filterOutput(output)))
		successLog("Benchmark completed successfully")
		
		// Clean up temporary folder
		debugLog("Cleaning up temporary folder %s", tmpFolder)
		err = os.RemoveAll(tmpFolder)
		if err != nil {
			debugLog("Warning: Failed to remove temporary folder: %s. Error: %s", tmpFolder, err)
		}
		
		return
	}

	// Otherwise, use Terraform to provision a new machine
	terraformDir := getTerraformDir()

	startSpinner("Initializing Terraform...")
	installer := &releases.ExactVersion{
		Product: product.Terraform,
		Version: version.Must(version.NewVersion("1.7.5")),
	}

	execPath, err := installer.Install(context.Background())
	if err != nil {
		stopSpinner()
		errorLog("Failed to install Terraform: %s", err)
		os.Exit(1)
	}

	// Initialize a new tfexec.Terraform object
	terraform, err := tfexec.NewTerraform(terraformDir, execPath)
	if err != nil {
		stopSpinner()
		errorLog("Failed to initialize Terraform: %s", err)
		os.Exit(1)
	}
	stopSpinner()

	debugLog("Initializing Terraform in %s", terraformDir)
	startSpinner("Initializing Terraform providers...")
	// Initialize with options compatible with Terraform 1.7.5
	err = terraform.Init(context.Background(), 
		tfexec.Upgrade(true),
		tfexec.ForceCopy(true),
	)
	stopSpinner()
	if err != nil {
		errorLog("Failed to initialize Terraform: %s", err)
		infoLog("Attempting to initialize with alternative options...")
		
		// Try again with minimal options
		startSpinner("Reinitializing Terraform...")
		err = terraform.Init(context.Background(), tfexec.Upgrade(true))
		stopSpinner()
		if err != nil {
			errorLog("Failed to initialize Terraform: %s", err)
			fmt.Println("\nTo fix this manually, try running: cd", terraformDir, "&& terraform init -upgrade")
			os.Exit(1)
		}
	}
	successLog("Terraform initialized successfully")

	// Skip show command which may not work properly before apply
	// _, err = terraform.Show(context.Background())
	// if err != nil && !strings.Contains(err.Error(), "state snapshot was created") {
	//	errorLog("Failed to run Terraform show: %s", err)
	//	os.Exit(1)
	// }

	buffer := &bytes.Buffer{}
	terraform.SetStdout(buffer)
	terraform.SetStderr(buffer)

	applyVars := []tfexec.ApplyOption{
		tfexec.Var("benchmark_folder=" + fullTempFolder),
		tfexec.Var("instance_type=" + *instanceType),
	}
	
	// Set the command to run
	applyVars = append(applyVars, tfexec.Var("custom_command=echo \"Remote-Output: $("+cmdToRun+")\""))
	
	startSpinner("Provisioning machine...")
	// Run 'terraform apply' in the given directory
	err = terraform.Apply(context.Background(), applyVars...)
	stopSpinner()
	if err != nil {
		errorLog("Error running terraform apply: %s.\n"+
			"⚠️  Although, an error occurred while running terraform apply, resources might have been created! Ensure to run:\n"+
			"cd %s && terraform destroy\n", err, terraformDir)
		os.Exit(1)
	}
	successLog("Machine provisioned successfully")
	fmt.Println(string(filterOutput(buffer.Bytes())))
	debugLog("Terraform apply completed successfully")

	// TODO: add schedule to destroy feature
	infoLog("Destroying provisioned machine...")
	
	// Create a context with timeout for the destroy operation
	destroyCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	
	startSpinner("Running terraform destroy (timeout: 3 minutes)...")
	
	// Capture stderr/stdout for debugging
	destroyBuffer := &bytes.Buffer{}
	terraform.SetStdout(destroyBuffer)
	terraform.SetStderr(destroyBuffer)
	
	// Run destroy with timeout context
	destroyErr := terraform.Destroy(destroyCtx, tfexec.Var("benchmark_folder=" + fullTempFolder),
		tfexec.Var("instance_type=" + *instanceType))
	stopSpinner()
	
	if destroyErr != nil {
		if errors.Is(destroyErr, context.DeadlineExceeded) {
			errorLog("Terraform destroy timed out after 3 minutes. Resources may still exist.")
			fmt.Printf("⚠️  Terraform destroy operation timed out. Resources might still exist! Manually destroy with:\n"+
				"cd %s && terraform destroy\n", terraformDir)
		} else {
			errorLog("Error running terraform destroy: %s", destroyErr)
			fmt.Printf("⚠️  Although, an error occurred while running terraform destroy, resources might have been created! Ensure to run:\n"+
				"cd %s && terraform destroy\n", terraformDir)
		}
		
		// Output buffer content to help diagnose the issue
		if debugMode {
			fmt.Println("Debug output from terraform destroy:")
			fmt.Println(destroyBuffer.String())
		}
		
		// We continue execution to clean up local resources even if destroy failed
	} else {
		successLog("Terraform resources destroyed successfully")
	}

	debugLog("Cleaning up temporary folder %s", fullTempFolder)
	err = os.RemoveAll(fullTempFolder)
	if err != nil {
		errorLog("Failed to remove the temporary folder: %s. Error: %s", fullTempFolder, err)
		os.Exit(1)
	}
	successLog("Benchmark completed successfully")
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Helper function to check if a file exists
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Helper function to check if a directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func printUsageAndExit() {
	fmt.Println("Usage: ib-agent-cli [options] [COMMAND] | [--command=\"custom command\"]")
	fmt.Println("\nOptions:")
	fmt.Println("  --host=IP               Run on existing machine with this IP address")
	fmt.Println("  --ssh-key=PATH          Path to SSH private key for connecting to existing machine")
	fmt.Println("  --ssh-user=USERNAME     SSH username for connecting to existing machine (default: ubuntu)")
	fmt.Println("  --folder=PATH           Path to folder containing all dependencies to be copied")
	fmt.Println("  --command=COMMAND       Custom command to run on the instance")
	fmt.Println("  --instance-type=TYPE    AWS instance type to use (default: t2.micro)")
	fmt.Println("  --debug                 Enable debug logging")
	os.Exit(1)
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
