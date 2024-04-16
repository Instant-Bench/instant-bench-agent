package main

import (
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

func main() {
	copyFS := flag.Bool("copy-fs", false, "Copy filesystem")
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		printUsageAndExit()
	}

	binary := args[0]
	entrypoint := args[1]

	binaryPath, err := exec.LookPath(binary)
	if err != nil {
		log.Fatalf("Binary %s not found in PATH", binary)
		os.Exit(1)
	}

	entrypointPath := entrypoint
	if !filepath.IsAbs(entrypoint) {
		cwd, err := os.Getwd()
		if err != nil {
			log.Println("Failed to get working directory")
			os.Exit(1)
		}

		entrypointPath, err = filepath.Abs(filepath.Join(cwd, entrypoint))
		if err != nil {
			log.Fatalf("Failed to get absolute path for entrypoint %s", entrypoint)
			os.Exit(1)
		}
	}

	tmpFolder, err := os.MkdirTemp(".", ".ib-")
	fmt.Printf("Created benchmark folder %s\n", tmpFolder)
	if err != nil {
		log.Fatalf("Failed to create temporary folder: %v", err)
	}

	if *copyFS {
		// Implement copying filesystem if required
	} else {
		binaryFullPath := filepath.Join(tmpFolder, filepath.Base(binaryPath))
		copyFile(binaryPath, binaryFullPath)
		copyFile(entrypointPath, filepath.Join(tmpFolder, filepath.Base(entrypointPath)))

		// TODO: this is not working
		err := os.Chmod(binaryFullPath, 0755)
		if err != nil {
			log.Fatalf("Failed to chmod binary: %v", err)
			os.Exit(1)
		}
	}

	// Path to the directory containing Terraform configuration files
	terraformDir, err := filepath.Abs("../aws")
	if err != nil {
		log.Fatalf("Failed to resolve aws folder: %v", err)
		os.Exit(1)
	}

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

	terraform.SetStdout(os.Stdout)
	terraform.SetStderr(os.Stderr)
	// Run 'terraform apply' in the given directory
	err = terraform.Apply(context.Background(), tfexec.Var("benchmark_folder="+fullTempFolder), tfexec.Var("instance_type=t2.micro"))
	if err != nil {
		fmt.Printf("Error running terraform apply: %s.\n"+
			"⚠️  Although, an error ocurred while running terraform apply, resources might have been created! Ensure to run:\n"+
			"cd %s && terraform destroy\n", err, terraformDir)
		os.Exit(1)
	}
	fmt.Println("Terraform apply completed successfully.")

	err = terraform.Destroy(context.Background(), tfexec.Var("benchmark_folder="+fullTempFolder), tfexec.Var("instance_type=t2.micro"))
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
	fmt.Println("Usage: ib $BINARY $ENTRYPOINT [--copy-fs]")
	os.Exit(1)
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
