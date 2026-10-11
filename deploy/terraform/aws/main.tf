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
# cannot be chosen per resource, so the regions are five fixed slots:
#   r1  the prover complex only (the prover host and its workers), us-east-1
#   r2  validators, eu-west-1
#   r3  validators, ap-southeast-1
#   r4  the peripheral networks only (Bitcoin regtest, celestia-app), us-west-2
#   r5  validators, us-east-2 (near the prover complex: the prover polls node01's RPC)
# validator_slots places the validators, infra_slot the peripheral networks, prover_slot the prover complex.
variable "regions" {
  type = object({ r1 = string, r2 = string, r3 = string, r4 = string, r5 = string })
  default = {
    r1 = "us-east-1"
    r2 = "eu-west-1"
    r3 = "ap-southeast-1"
    r4 = "us-west-2"
    r5 = "us-east-2"
  }
}

variable "validator_slots" {
  type = list(string)
  # node01 and node02 are in r5, the closest validators to the prover complex: the prover polls node01's RPC.
  default = ["r5", "r5", "r2", "r2", "r3", "r3", "r3"]
  validation {
    condition     = length(var.validator_slots) >= 4 && alltrue([for s in var.validator_slots : contains(["r1", "r2", "r3", "r4", "r5"], s)])
    error_message = "Need at least 4 validators, each in slot r1..r5."
  }
}

variable "infra_slot" {
  type        = string
  default     = "r4"
  description = "Region slot of the peripheral networks (infra01)."
}

variable "prover_slot" {
  type        = string
  default     = "r1"
  description = "Region slot of the prover host and its workers."
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
  default = "m5.large" # 2 vCPU / 8 GB, fixed-performance CPU (t3 is burstable and can throttle)
}

variable "infra_instance_type" {
  type    = string
  default = "c5.2xlarge" # 8 vCPU / 16 GB: Bitcoin regtest, miner and celestia-app
}

variable "prover_dedicated" {
  type        = bool
  default     = true
  description = "Run the ZK prover on its own host, in the infra slot, so that proving does not compete with Bitcoin and celestia-app for CPU."
}

variable "prover_instance_type" {
  type    = string
  default = "c7a.2xlarge" # 8 vCPU / 16 GB: one proof peaks at about 7.5 GB and nargo execute is single-threaded, so the fastest single core wins (a proof takes 65 s here, 111 s on a c5.2xlarge)
}

variable "prover_coordinator_instance_type" {
  type        = string
  default     = "c7a.xlarge" # 4 vCPU / 8 GB: with workers the prover host only prepares witnesses and submits proofs
  description = "Instance type of the prover host when it has workers (prover_instance_type is used when it proves by itself)."
}

variable "prover_workers" {
  type        = number
  default     = 0
  description = "Number of proving workers. With 0 the prover host proves by itself, one proof at a time. With N the prover host only prepares and submits proofs and hands each proof to a worker, so N proofs are in flight; every worker is in the infra slot."
}

variable "prover_worker_instance_type" {
  type    = string
  default = "c7a.2xlarge"
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
provider "aws" {
  alias  = "r5"
  region = var.regions.r5
}

locals {
  public_key = trimspace(file(pathexpand(var.ssh_public_key_path)))
  vms = merge(
    { infra01 = { slot = var.infra_slot, instance_type = var.infra_instance_type, volume_gb = 100 } },
    var.prover_dedicated ? { prover01 = { slot = var.prover_slot, instance_type = var.prover_workers > 0 ? var.prover_coordinator_instance_type : var.prover_instance_type, volume_gb = 40 } } : {},
    { for i in range(var.prover_workers) :
      format("prover-worker%02d", i + 1) => { slot = var.prover_slot, instance_type = var.prover_worker_instance_type, volume_gb = 40 }
    },
    { for i, s in var.validator_slots :
      format("engram-node%02d", i + 1) => { slot = s, instance_type = var.validator_instance_type, volume_gb = 40 }
    },
  )
  by_slot = { for s in ["r1", "r2", "r3", "r4", "r5"] :
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

module "r4" {
  source         = "./modules/region"
  providers      = { aws = aws.r4 }
  slot           = "r4"
  name_prefix    = var.name_prefix
  public_key     = local.public_key
  operator_cidrs = var.operator_cidrs
  vms            = local.by_slot.r4
}

module "r5" {
  source         = "./modules/region"
  providers      = { aws = aws.r5 }
  slot           = "r5"
  name_prefix    = var.name_prefix
  public_key     = local.public_key
  operator_cidrs = var.operator_cidrs
  vms            = local.by_slot.r5
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

locals {
  ips         = merge(module.r1.ips, module.r2.ips, module.r3.ips, module.r4.ips, module.r5.ips)
  private_ips = merge(module.r1.private_ips, module.r2.private_ips, module.r3.private_ips, module.r4.private_ips, module.r5.private_ips)
}

output "inventory" {
  description = "Read by deploy/bin/make-inventory.py."
  value = {
    ssh_user = "ubuntu"
    infra    = { name = "infra01", ip = local.ips["infra01"], region = var.regions[var.infra_slot] }
    prover   = var.prover_dedicated ? { name = "prover01", ip = local.ips["prover01"], private_ip = local.private_ips["prover01"], region = var.regions[var.prover_slot] } : null
    prover_workers = [
      for i in range(var.prover_workers) : {
        name       = format("prover-worker%02d", i + 1)
        ip         = local.ips[format("prover-worker%02d", i + 1)]
        private_ip = local.private_ips[format("prover-worker%02d", i + 1)]
        region     = var.regions[var.prover_slot]
      }
    ]
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
