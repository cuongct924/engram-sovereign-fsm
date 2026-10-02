terraform {
  required_version = ">= 1.5"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

# Authenticate with `gcloud auth application-default login` (or GOOGLE_APPLICATION_CREDENTIALS).
provider "google" {
  project = var.project
}

variable "project" {
  type        = string
  description = "GCP project id."
}

variable "name_prefix" {
  type    = string
  default = "engram"
}

variable "ssh_user" {
  type    = string
  default = "ubuntu"
}

variable "ssh_public_key_path" {
  type    = string
  default = "~/.ssh/id_ed25519.pub"
}

variable "operator_cidrs" {
  type        = list(string)
  description = "CIDRs allowed to SSH in and reach the validator RPC and the infra Bitcoin RPC."
}

# One zone per validator; the region is derived from the zone.
variable "validator_zones" {
  type    = list(string)
  default = ["us-east1-b", "us-east1-b", "europe-west1-b", "europe-west1-b", "asia-southeast1-b", "southamerica-east1-b", "australia-southeast1-b"]
  validation {
    condition     = length(var.validator_zones) >= 4
    error_message = "Need at least 4 validators."
  }
}

variable "infra_zone" {
  type    = string
  default = "us-east1-b"
}

variable "validator_machine_type" {
  type    = string
  default = "e2-medium" # 2 vCPU / 4 GB
}

variable "infra_machine_type" {
  type    = string
  default = "e2-standard-4" # 4 vCPU / 16 GB: Bitcoin, celestia-app and the ZK prover
}

locals {
  validators = {
    for i, z in var.validator_zones :
    format("engram-node%02d", i + 1) => { index = i + 1, zone = z }
  }
  ssh_key = "${var.ssh_user}:${trimspace(file(pathexpand(var.ssh_public_key_path)))}"
}

resource "google_compute_network" "engram" {
  name                    = "${var.name_prefix}-net"
  auto_create_subnetworks = true
}

# Cloud-level rules; the Ansible `common` role restricts every port to the peer IPs with ufw.
resource "google_compute_firewall" "ssh" {
  name          = "${var.name_prefix}-ssh"
  network       = google_compute_network.engram.name
  source_ranges = var.operator_cidrs
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
  target_tags = ["engram"]
}

resource "google_compute_firewall" "services" {
  name          = "${var.name_prefix}-services"
  network       = google_compute_network.engram.name
  source_ranges = ["0.0.0.0/0"]
  allow {
    protocol = "tcp"
    ports    = ["26656", "26657", "18443", "9090"]
  }
  target_tags = ["engram"]
}

resource "google_compute_instance" "infra" {
  name         = "${var.name_prefix}-infra01"
  machine_type = var.infra_machine_type
  zone         = var.infra_zone
  tags         = ["engram"]

  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2404-lts-amd64"
      size  = 100
    }
  }
  network_interface {
    network = google_compute_network.engram.name
    access_config {}
  }
  metadata = { ssh-keys = local.ssh_key }
}

resource "google_compute_instance" "validator" {
  for_each     = local.validators
  name         = "${var.name_prefix}-${each.key}"
  machine_type = var.validator_machine_type
  zone         = each.value.zone
  tags         = ["engram"]

  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2404-lts-amd64"
      size  = 40
    }
  }
  network_interface {
    network = google_compute_network.engram.name
    access_config {}
  }
  metadata = { ssh-keys = local.ssh_key }
}

output "inventory" {
  description = "Read by deploy/bin/make-inventory.py."
  value = {
    ssh_user = var.ssh_user
    infra = {
      name   = "infra01"
      ip     = google_compute_instance.infra.network_interface[0].access_config[0].nat_ip
      region = var.infra_zone
    }
    validators = [
      for name, v in local.validators : {
        name   = name
        index  = v.index
        ip     = google_compute_instance.validator[name].network_interface[0].access_config[0].nat_ip
        region = v.zone
      }
    ]
  }
}
