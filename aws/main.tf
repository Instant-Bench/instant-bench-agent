terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.0"
    }
  }
}

provider "aws" {
  region = "us-east-1"
}

data "aws_ami" "ubuntu" {
  most_recent = true

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  owners = ["099720109477"] # Canonical
}

resource "tls_private_key" "example" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "aws_key_pair" "generated_key" {
  key_name   = "cloudtls"
  public_key = tls_private_key.example.public_key_openssh
}

resource "aws_security_group" "security" {
  name = "allow-all"

  ingress {
    cidr_blocks = [
      "0.0.0.0/0"
    ]
    from_port = 22
    to_port   = 22
    protocol  = "tcp"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = -1
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "example" {
  ami                         = data.aws_ami.ubuntu.id
  instance_type               = var.instance_type
  key_name                    = aws_key_pair.generated_key.key_name
  vpc_security_group_ids      = [aws_security_group.security.id]
  associate_public_ip_address = true

  tags = {
    Name = "instant-bench"
  }

  # this copies the files in the ec2_files/ directory to /home/ec2-user on the instance
  provisioner "file" {
    source      = var.benchmark_folder
    destination = "/home/ubuntu/benchmark"
  }

  # this is required to establish a connection and to copy files to the EC2 instance id from local disk
  connection {
    type        = "ssh"
    user        = "ubuntu"
    private_key = tls_private_key.example.private_key_pem
    host        = self.public_ip
  }

  provisioner "remote-exec" {
    inline = [
      "cd /home/ubuntu/benchmark",

      "git clone https://github.com/google/benchmark.git",
      "cd benchmark",
      "cmake -E make_directory \"build\"",
      "cmake -E chdir \“build\” cmake -DBENCHMARK_DOWNLOAD_DEPENDENCIES=on -DCMAKE_BUILD_TYPE=Release ../",
      "cmake — build “build” — config Release",
      var.remove_script
    ]
  }
}

