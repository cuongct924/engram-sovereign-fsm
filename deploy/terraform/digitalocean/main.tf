terraform {
  required_version = ">= 1.5"
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.40"
    }
  }
}

# Authenticate with the DIGITALOCEAN_TOKEN environment variable.
provider "digitalocean" {}

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
  description = "CIDRs allowed to SSH in and reach the validator RPC and the infra Bitcoin RPC."
}

# DigitalOcean regions: nyc3, sfo3, ams3, fra1, sgp1, blr1, syd1 ...
variable "validator_regions" {
  type    = list(string)
  default = ["nyc3", "nyc3", "ams3", "fra1", "sfo3", "sgp1", "blr1"]
  validation {
    condition     = length(var.validator_regions) >= 4
    error_message = "Need at least 4 validators."
  }
}

variable "infra_region" {
  type    = string
  default = "nyc3"
}

variable "validator_size" {
  type    = string
  default = "s-2vcpu-4gb"
}

variable "infra_size" {
  type    = string
  default = "s-8vcpu-16gb" # Bitcoin, celestia-app and the ZK prover
}

variable "image" {
  type    = string
  default = "ubuntu-24-04-x64"
}

locals {
  validators = {
    for i, r in var.validator_regions :
    format("engram-node%02d", i + 1) => { index = i + 1, region = r }
  }
}

resource "digitalocean_ssh_key" "engram" {
  name       = "${var.name_prefix}-key"
  public_key = file(pathexpand(var.ssh_public_key_path))
}

resource "digitalocean_droplet" "infra" {
  name     = "${var.name_prefix}-infra01"
  region   = var.infra_region
  size     = var.infra_size
  image    = var.image
  ssh_keys = [digitalocean_ssh_key.engram.fingerprint]
}

resource "digitalocean_droplet" "validator" {
  for_each = local.validators
  name     = "${var.name_prefix}-${each.key}"
  region   = each.value.region
  size     = var.validator_size
  image    = var.image
  ssh_keys = [digitalocean_ssh_key.engram.fingerprint]
}

# Cloud-level rules; the Ansible `common` role additionally restricts every port to the peer IPs
# with ufw on each host.
resource "digitalocean_firewall" "engram" {
  name = "${var.name_prefix}-fw"
  droplet_ids = concat(
    [digitalocean_droplet.infra.id],
    [for d in digitalocean_droplet.validator : d.id],
  )

  inbound_rule {
    protocol         = "tcp"
    port_range       = "22"
    source_addresses = var.operator_cidrs
  }
  dynamic "inbound_rule" {
    for_each = ["26656", "26657", "18443", "9090"]
    content {
      protocol         = "tcp"
      port_range       = inbound_rule.value
      source_addresses = ["0.0.0.0/0"]
    }
  }
  outbound_rule {
    protocol              = "tcp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
  outbound_rule {
    protocol              = "udp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
  outbound_rule {
    protocol              = "icmp"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
}

output "inventory" {
  description = "Read by deploy/bin/make-inventory.py."
  value = {
    ssh_user = "root"
    infra    = { name = "infra01", ip = digitalocean_droplet.infra.ipv4_address, region = var.infra_region }
    validators = [
      for name, v in local.validators : {
        name   = name
        index  = v.index
        ip     = digitalocean_droplet.validator[name].ipv4_address
        region = v.region
      }
    ]
  }
}
