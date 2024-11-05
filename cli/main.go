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

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
)

func filterOutput(output []byte) []byte {
	lines := bytes.Split(output, []byte("\n"))
	var filteredOutput []byte
	for _, line := range lines {
		if bytes.Contains(line, []byte("Remote-Output")) {
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

func main() {
	copyFS := flag.Bool("copy-fs", false, "Copy filesystem")
	command := flag.String("command", "", "Custom command to run on the AWS instance")
	binaryFlag := flag.String("binary", "", "Path to binary file to copy to the AWS instance if no positional arguments are provided")
	// compileCmd := flag.String("compile-cmd", "", "Compile command")
	flag.Parse()

	args := flag.Args()
	var binaryPath, entrypointPath string

	if len(args) == 2 {
		// Original mode: binary as first argument, entrypoint as second argument
		binary := args[0]
		entrypoint := args[1]

		var err error
		binaryPath, err = exec.LookPath(binary)
		if err != nil {
			log.Fatalf("Binary %s not found in PATH", binary)
		}

		if !filepath.IsAbs(entrypoint) {
			cwd, err := os.Getwd()
			if err != nil {
				log.Println("Failed to get working directory")
				os.Exit(1)
			}
			entrypointPath, err = filepath.Abs(filepath.Join(cwd, entrypoint))
			if err != nil {
				log.Fatalf("Failed to get absolute path for entrypoint %s", entrypoint)
			}
		} else {
			entrypointPath = entrypoint
		}
	} else if *command != "" && *binaryFlag != "" {
		// New mode: --command and --binary provided without positional arguments
		var err error
		binaryPath, err = filepath.Abs(*binaryFlag)
		if err != nil {
			log.Fatalf("Failed to get absolute path for binary %s", *binaryFlag)
		}
	} else {
		printUsageAndExit()
	}
	
	tmpFolder, err := os.MkdirTemp(".", ".ib-")
	fmt.Printf("Created benchmark folder %s\n", tmpFolder)
	if err != nil {
		log.Fatalf("Failed to create temporary folder: %v", err)
	}

	if *copyFS {
		// TODO: Implement copying filesystem if required
	} else {
		binaryFullPath := filepath.Join(tmpFolder, filepath.Base(binaryPath))
		copyFile(binaryPath, binaryFullPath)
		if entrypointPath != "" {
			copyFile(entrypointPath, filepath.Join(tmpFolder, filepath.Base(entrypointPath)))
		}
	}

	terraformDir := getTerraformDir()
	fullTempFolder, err := filepath.Abs(tmpFolder)
	if err != nil {
		log.Fatalf("Failed to resolve temporary folder: %v", err)
		os.Exit(1)
	}

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
		tfexec.Var("instance_type=t2.micro"),
	}
	if *command != "" {
		applyVars = append(applyVars, tfexec.Var("custom_command=echo \"Remote-Output: $("+*command+")\""))
	}
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
		tfexec.Var("instance_type=t2.micro"))
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

func printUsageAndExit() {
	fmt.Println("Usage: ib [BINARY ENTRYPOINT] | [--binary=BINARY_PATH --command=\"custom command\"]")
}

func copyFile(src, dst string) {
	srcFile, err := os.Open(src)
	if err != nil {
		log.Fatalf("Failed to open source file %s: %v", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		log.Fatalf("Failed to create destination file %s: %v", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		log.Fatalf("Failed to copy data from %s to %s: %v", src, dst, err)
	}

}
