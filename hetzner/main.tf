terraform {
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.48"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
    null = {
      source  = "hashicorp/null"
      version = "~> 3.2"
    }
  }
}

# The hcloud provider reads the token from the environment variable HCLOUD_TOKEN by default.
# Alternatively, you can set: token = var.hcloud_token
provider "hcloud" {}

# Generate an SSH key pair for provisioning
resource "tls_private_key" "example" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

# Upload the public key to Hetzner Cloud
resource "hcloud_ssh_key" "generated_key" {
  name       = "cloudtls"
  public_key = tls_private_key.example.public_key_openssh
}

# Create a server
resource "hcloud_server" "server" {
  name        = "instant-bench"
  server_type = var.server_type
  image       = "ubuntu-22.04"
  location    = var.location
  ssh_keys    = [hcloud_ssh_key.generated_key.id]

  # Wait for IPv4; hcloud gives public IPv4 by default
}

# Copy benchmark files to the server
resource "null_resource" "provision" {
  # Re-run provisioners when the input folder changes (best-effort)
  triggers = {
    benchmark_folder = var.benchmark_folder
    custom_command   = var.custom_command
    server_id        = hcloud_server.server.id
  }

  connection {
    type        = "ssh"
    user        = "root"
    private_key = tls_private_key.example.private_key_pem
    host        = hcloud_server.server.ipv4_address
  }

  provisioner "file" {
    source      = var.benchmark_folder
    destination = "/root/benchmark"
  }

  provisioner "remote-exec" {
    inline = [
      "cd /root/benchmark",
      "curl -o- -s https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash > /dev/null 2>&1",
      ". ~/.nvm/nvm.sh > /dev/null 2>&1",
      "nvm install v22 > /dev/null 2>&1",
      "echo 'BENCHMARK_START'",
      "echo 'Run 1'",
      var.custom_command,
      "echo 'Run 2'",
      var.custom_command,
      "echo 'Run 3'",
      var.custom_command,
      "echo 'BENCHMARK_END'",
    ]
    on_failure = continue
  }
}
