variable "server_type" {
  type        = string
  description = "Hetzner Cloud server type (e.g., cx22, cpx11, cax11)."
  default     = "cax11"
}

variable "location" {
  type        = string
  description = "Hetzner Cloud location (e.g., fsn1, hel1, nbg1)."
  default     = "fsn1"
}

variable "benchmark_folder" {
  type        = string
  description = "The folder with files to run on the remote machine"
}

variable "custom_command" {
  type        = string
  default     = ""
}
