terraform {
  required_version = ">= 1.5"
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.48"
    }
  }
}

# Authenticate with the HCLOUD_TOKEN environment variable.
provider "hcloud" {}

variable "name_prefix" {
  type    = string
  default = "engram"
}

variable "ssh_public_key_path" {
  type    = string
  default = "~/.ssh/id_ed25519.pub"
}

variable "operator_cidrs" {
  type        = list(string)
  description = "CIDRs allowed to SSH in and reach the validator RPC and the infra Bitcoin RPC (the machine running the experiments)."
}

# One entry per validator, in node order. Hetzner locations: fsn1, nbg1, hel1 (EU), ash, hil (US), sin (Asia).
variable "validator_locations" {
  type    = list(string)
  default = ["fsn1", "hel1", "ash", "ash", "hil", "sin", "sin"]
  validation {
    condition     = length(var.validator_locations) >= 4
    error_message = "Need at least 4 validators."
  }
}

variable "infra_location" {
  type    = string
  default = "ash"
}

# cpx types exist in every location; the cx line is EU-only.
variable "validator_server_type" {
  type    = string
  default = "cpx21"
}

variable "infra_server_type" {
  type    = string
  default = "cpx41" # 8 vCPU / 16 GB: Bitcoin, celestia-app and the ZK prover
}

variable "image" {
  type    = string
  default = "ubuntu-24.04"
}

locals {
  validators = {
    for i, loc in var.validator_locations :
    format("engram-node%02d", i + 1) => { index = i + 1, location = loc }
  }
}

resource "hcloud_ssh_key" "engram" {
  name       = "${var.name_prefix}-key"
  public_key = file(pathexpand(var.ssh_public_key_path))
}

resource "hcloud_server" "infra" {
  name        = "${var.name_prefix}-infra01"
  server_type = var.infra_server_type
  image       = var.image
  location    = var.infra_location
  ssh_keys    = [hcloud_ssh_key.engram.id]
}

resource "hcloud_server" "validator" {
  for_each    = local.validators
  name        = "${var.name_prefix}-${each.key}"
  server_type = var.validator_server_type
  image       = var.image
  location    = each.value.location
  ssh_keys    = [hcloud_ssh_key.engram.id]
}

locals {
  host_ips = concat(
    [hcloud_server.infra.ipv4_address],
    [for s in hcloud_server.validator : s.ipv4_address],
  )
  host_cidrs = [for ip in local.host_ips : "${ip}/32"]
}

# Tight cloud-level rules: the hosts talk to each other, the operator reaches SSH and the RPCs.
resource "hcloud_firewall" "engram" {
  name = "${var.name_prefix}-fw"

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = var.operator_cidrs
  }
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "26656"
    source_ips = local.host_cidrs
  }
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "26657"
    source_ips = concat(local.host_cidrs, var.operator_cidrs)
  }
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "18443"
    source_ips = concat(local.host_cidrs, var.operator_cidrs)
  }
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "9090"
    source_ips = local.host_cidrs
  }
}

resource "hcloud_firewall_attachment" "engram" {
  firewall_id = hcloud_firewall.engram.id
  server_ids  = concat([hcloud_server.infra.id], [for s in hcloud_server.validator : s.id])
}

output "inventory" {
  description = "Read by deploy/bin/make-inventory.py."
  value = {
    ssh_user = "root"
    infra    = { name = "infra01", ip = hcloud_server.infra.ipv4_address, region = var.infra_location }
    validators = [
      for name, v in local.validators : {
        name   = name
        index  = v.index
        ip     = hcloud_server.validator[name].ipv4_address
        region = v.location
      }
    ]
  }
}
