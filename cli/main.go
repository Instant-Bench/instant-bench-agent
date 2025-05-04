package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
)

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
	flag.Parse()

	args := flag.Args()
	var binaryPath string
	var cmdToRun string
	var filesToCopy []string

	if len(args) == 1 {
		// Single positional argument is treated as a command
		cmdToRun = args[0]
		// Try to infer binary from command
		cmdParts := strings.Fields(cmdToRun)
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
		cmdToRun = strings.Join(args, " ")
	} else if *command != "" {
		// Command flag is provided
		cmdToRun = *command
		// Try to infer binary from command
		cmdParts := strings.Fields(cmdToRun)
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
	
	// Find files to copy
	cmdParts := strings.Fields(cmdToRun)
	for _, part := range cmdParts {
		// Skip the binary and any flags
		if strings.HasPrefix(part, "-") || (len(cmdParts) > 0 && part == cmdParts[0]) {
			continue
		}
		
		// Check if this part looks like a file path
		if strings.Contains(part, ".") && !strings.HasPrefix(part, "-") {
			// Remove any quotes
			part = strings.Trim(part, "'\"")
			
			// Check if the file exists
			if _, err := os.Stat(part); err == nil {
				// It's a file, add it to the list
				absPath, err := filepath.Abs(part)
				if err == nil {
					filesToCopy = append(filesToCopy, absPath)
				}
			}
		}
	}
	
	tmpFolder, err := os.MkdirTemp(".", ".ib-")
	fmt.Printf("Created benchmark folder %s\n", tmpFolder)
	if err != nil {
		log.Fatalf("Failed to create temporary folder: %v", err)
	}

	// Copy files to temporary folder
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
		err = copyDir(absPath, tmpFolder)
		if err != nil {
			log.Fatalf("Failed to copy folder: %v", err)
		}
		
		// Adjust the command to use the correct paths in the remote environment
		cmdParts := strings.Fields(cmdToRun)
		if len(cmdParts) > 0 {
			// Keep the binary name the same
			newCmd := []string{cmdParts[0]}
			
			// Adjust paths for other arguments
			for i := 1; i < len(cmdParts); i++ {
				part := cmdParts[i]
				if strings.HasPrefix(part, *folderPath) {
					// If the path starts with the folder path, replace it with the relative path
					relPath, err := filepath.Rel(*folderPath, part)
					if err == nil {
						newCmd = append(newCmd, relPath)
						continue
					}
				}
				newCmd = append(newCmd, part)
			}
			
			cmdToRun = strings.Join(newCmd, " ")
		}
	} else {
		// Copy binary if it exists
		if binaryPath != "" {
			binaryFullPath := filepath.Join(tmpFolder, filepath.Base(binaryPath))
			err = copyFile(binaryPath, binaryFullPath)
			if err != nil {
				log.Fatalf("Failed to copy binary: %v", err)
			}
		}
		
		// Process each file to copy
		remappedPaths := make(map[string]string)
		for _, file := range filesToCopy {
			// Get the file's directory
			fileName := filepath.Base(file)
			
			// Create the destination path in the temp folder
			destPath := filepath.Join(tmpFolder, fileName)
			
			// Copy the file
			err = copyFile(file, destPath)
			if err != nil {
				log.Printf("Warning: Failed to copy file %s: %v", file, err)
			} else {
				fmt.Printf("Copied file: %s to %s\n", file, destPath)
				remappedPaths[file] = fileName
			}
		}
		
		// Adjust the command to use the correct paths in the remote environment
		cmdParts := strings.Fields(cmdToRun)
		if len(cmdParts) > 0 {
			// Keep the binary name the same
			newCmd := []string{cmdParts[0]}
			
			// Adjust paths for other arguments
			for i := 1; i < len(cmdParts); i++ {
				part := cmdParts[i]
				// Remove any quotes
				part = strings.Trim(part, "'\"")
				
				// Check if this is a file path we've copied
				absPath, err := filepath.Abs(part)
				if err == nil && remappedPaths[absPath] != "" {
					// Replace with just the file name
					newCmd = append(newCmd, remappedPaths[absPath])
				} else {
					// Keep as is
					newCmd = append(newCmd, part)
				}
			}
			
			cmdToRun = strings.Join(newCmd, " ")
			fmt.Printf("Adjusted command for remote environment: %s\n", cmdToRun)
		}
	}

	fullTempFolder, err := filepath.Abs(tmpFolder)
	if err != nil {
		log.Fatalf("Failed to resolve temporary folder: %v", err)
		os.Exit(1)
	}

	// Run on existing machine if specified
	if *useExistingMachine != "" {
		fmt.Printf("Running benchmark on existing machine: %s\n", *useExistingMachine)
		
		// Validate SSH key if provided
		if *sshKeyPath != "" {
			if _, err := os.Stat(*sshKeyPath); os.IsNotExist(err) {
				log.Fatalf("SSH key file not found: %s", *sshKeyPath)
			}
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
		
		// Copy files to remote machine
		sshKeyOption := ""
		if *sshKeyPath != "" {
			sshKeyOption = "-i " + *sshKeyPath
		}
		
		// Create benchmark directory on remote machine
		createDirCmd := exec.Command("ssh", strings.Split(sshKeyOption+" "+*sshUser+"@"+*useExistingMachine+" mkdir -p /home/"+*sshUser+"/benchmark", " ")...)
		createDirCmd.Stdout = os.Stdout
		createDirCmd.Stderr = os.Stderr
		err = createDirCmd.Run()
		if err != nil {
			log.Fatalf("Failed to create benchmark directory on remote machine: %v", err)
		}
		
		// Copy files to remote machine
		scpCmd := exec.Command("scp", append(strings.Split(sshKeyOption+" -r ", " "), 
			fullTempFolder+"/*", *sshUser+"@"+*useExistingMachine+":/home/"+*sshUser+"/benchmark/")...)
		scpCmd.Stdout = os.Stdout
		scpCmd.Stderr = os.Stderr
		err = scpCmd.Run()
		if err != nil {
			log.Fatalf("Failed to copy files to remote machine: %v", err)
		}
		
		// Run benchmark script on remote machine
		sshCmd := exec.Command("ssh", strings.Split(sshKeyOption+" "+*sshUser+"@"+*useExistingMachine+" bash /home/"+*sshUser+"/benchmark/run_benchmark.sh", " ")...)
		output, err := sshCmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to run benchmark on remote machine: %v\nOutput: %s", err, output)
		}
		
		fmt.Println(string(filterOutput(output)))
		
		// Clean up temporary folder
		err = os.RemoveAll(fullTempFolder)
		if err != nil {
			fmt.Printf("An error ocurred when removing the temporary folder: %s. Error: %s", fullTempFolder, err)
		}
		
		return
	}

	// Otherwise, use Terraform to provision a new machine
	terraformDir := getTerraformDir()

	installer := &releases.ExactVersion{
		Product: product.Terraform,
		Version: version.Must(version.NewVersion("1.7.5")),
	}

	execPath, err := installer.Install(context.Background())
	if err != nil {
		log.Fatalf("error installing Terraform: %s", err)
	}

	// Initialize a new tfexec.Terraform object
	terraform, err := tfexec.NewTerraform(terraformDir, execPath)
	if err != nil {
		fmt.Printf("Error initializing Terraform: %s\n", err)
		os.Exit(1)
	}

	err = terraform.Init(context.Background(), tfexec.Upgrade(true))
	if err != nil {
		log.Fatalf("error running Init: %s", err)
		os.Exit(1)
	}

	_, err = terraform.Show(context.Background())
	if err != nil {
		log.Fatalf("error running Show: %s", err)
		os.Exit(1)
	}

	buffer := &bytes.Buffer{}
	terraform.SetStdout(buffer)
	terraform.SetStderr(buffer)
	// terraform.SetStdout(os.Stdout)
	// terraform.SetStderr(os.Stderr)

	applyVars := []tfexec.ApplyOption{
		tfexec.Var("benchmark_folder=" + fullTempFolder),
		tfexec.Var("instance_type=" + *instanceType),
	}
	
	// Set the command to run
	applyVars = append(applyVars, tfexec.Var("custom_command=echo \"Remote-Output: $("+cmdToRun+")\""))
	
	fmt.Println("Provisioning machine...")
	// Run 'terraform apply' in the given directory
	err = terraform.Apply(context.Background(), applyVars...)
	if err != nil {
		fmt.Printf("Error running terraform apply: %s.\n"+
			"⚠️  Although, an error ocurred while running terraform apply, resources might have been created! Ensure to run:\n"+
			"cd %s && terraform destroy\n", err, terraformDir)
		os.Exit(1)
	}
	fmt.Println(string(filterOutput(buffer.Bytes())))
	fmt.Println("Terraform apply completed successfully.")

	// TODO: add schedule to destroy feature
	fmt.Println("Destroying provisioned machine...")
	err = terraform.Destroy(context.Background(), tfexec.Var("benchmark_folder=" + fullTempFolder),
		tfexec.Var("instance_type=" + *instanceType))
	if err != nil {
		fmt.Printf("Error running terraform destroy: %s.\n"+
			"⚠️  Although, an error ocurred while running terraform destroy, resources might have been created! Ensure to run:\n"+
			"cd %s && terraform destroy\n", err, terraformDir)
		os.Exit(1)
	}
	fmt.Println("Terraform destroy completed successfully.")

	err = os.RemoveAll(fullTempFolder)
	if err != nil {
		fmt.Printf("An error ocurred when removing the temporary folder: %s. Error: %s", fullTempFolder, err)
		os.Exit(1)
	}
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

func printUsageAndExit() {
	fmt.Println("Usage: ib-agent-cli [options] [COMMAND] | [--command=\"custom command\"]")
	fmt.Println("\nOptions:")
	fmt.Println("  --host=IP               Run on existing machine with this IP address")
	fmt.Println("  --ssh-key=PATH          Path to SSH private key for connecting to existing machine")
	fmt.Println("  --ssh-user=USERNAME     SSH username for connecting to existing machine (default: ubuntu)")
	fmt.Println("  --folder=PATH           Path to folder containing all dependencies to be copied")
	fmt.Println("  --command=COMMAND       Custom command to run on the instance")
	fmt.Println("  --instance-type=TYPE    AWS instance type to use (default: t2.micro)")
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
