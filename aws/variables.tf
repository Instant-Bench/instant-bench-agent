variable "instance_type" {
  type        = string
  description = "The instance type to use for the instance."
}

variable "benchmark_folder" {
  type        = string
  description = "The folder with a ./node & index.js to run"
}

variable "remote_script" {
  type = string
  description = "Script to run the benchmark"
  default = "chmod +x ./node && ./node bench.js"
}
