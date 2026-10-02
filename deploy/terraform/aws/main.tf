terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# Authenticate with the usual AWS_* environment variables or a profile. A provider's region
# cannot be chosen per resource, so the regions are four fixed slots: put validators in a slot
# with validator_slots, and point two slots at the same region to use fewer regions.
variable "regions" {
  type = object({ r1 = string, r2 = string, r3 = string, r4 = string })
  default = {
    r1 = "us-east-1"
    r2 = "eu-west-1"
    r3 = "ap-southeast-1"
    r4 = "sa-east-1"
  }
}

variable "validator_slots" {
  type    = list(string)
  default = ["r1", "r1", "r2", "r2", "r3", "r3", "r4"]
  validation {
    condition     = length(var.validator_slots) >= 4 && alltrue([for s in var.validator_slots : contains(["r1", "r2", "r3", "r4"], s)])
    error_message = "Need at least 4 validators, each in slot r1..r4."
  }
}

variable "infra_slot" {
  type    = string
  default = "r1"
}

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

variable "validator_instance_type" {
  type    = string
  default = "t3.medium" # 2 vCPU / 4 GB
}

variable "infra_instance_type" {
  type    = string
  default = "t3.xlarge" # 4 vCPU / 16 GB: Bitcoin, celestia-app and the ZK prover
}

provider "aws" {
  alias  = "r1"
  region = var.regions.r1
}
provider "aws" {
  alias  = "r2"
  region = var.regions.r2
}
provider "aws" {
  alias  = "r3"
  region = var.regions.r3
}
provider "aws" {
  alias  = "r4"
  region = var.regions.r4
}

locals {
  public_key = trimspace(file(pathexpand(var.ssh_public_key_path)))
  vms = merge(
    { infra01 = { slot = var.infra_slot, instance_type = var.infra_instance_type, volume_gb = 100 } },
    { for i, s in var.validator_slots :
      format("engram-node%02d", i + 1) => { slot = s, instance_type = var.validator_instance_type, volume_gb = 40 }
    },
  )
  by_slot = { for s in ["r1", "r2", "r3", "r4"] :
    s => { for k, v in local.vms : k => { instance_type = v.instance_type, volume_gb = v.volume_gb } if v.slot == s }
  }
}

module "r1" {
  source         = "./modules/region"
  providers      = { aws = aws.r1 }
  slot           = "r1"
  name_prefix    = var.name_prefix
  public_key     = local.public_key
  operator_cidrs = var.operator_cidrs
  vms            = local.by_slot.r1
}

module "r2" {
  source         = "./modules/region"
  providers      = { aws = aws.r2 }
  slot           = "r2"
  name_prefix    = var.name_prefix
  public_key     = local.public_key
  operator_cidrs = var.operator_cidrs
  vms            = local.by_slot.r2
}

module "r3" {
  source         = "./modules/region"
  providers      = { aws = aws.r3 }
  slot           = "r3"
  name_prefix    = var.name_prefix
  public_key     = local.public_key
  operator_cidrs = var.operator_cidrs
  vms            = local.by_slot.r3
}

module "r4" {
  source         = "./modules/region"
  providers      = { aws = aws.r4 }
  slot           = "r4"
  name_prefix    = var.name_prefix
  public_key     = local.public_key
  operator_cidrs = var.operator_cidrs
  vms            = local.by_slot.r4
}

locals {
  ips = merge(module.r1.ips, module.r2.ips, module.r3.ips, module.r4.ips)
}

output "inventory" {
  description = "Read by deploy/bin/make-inventory.py."
  value = {
    ssh_user = "ubuntu"
    infra    = { name = "infra01", ip = local.ips["infra01"], region = var.regions[var.infra_slot] }
    validators = [
      for i, s in var.validator_slots : {
        name   = format("engram-node%02d", i + 1)
        index  = i + 1
        ip     = local.ips[format("engram-node%02d", i + 1)]
        region = var.regions[s]
      }
    ]
  }
}
