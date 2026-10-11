terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }
}

variable "slot" {
  type = string
}

variable "name_prefix" {
  type = string
}

variable "public_key" {
  type = string
}

variable "operator_cidrs" {
  type = list(string)
}

variable "vms" {
  type = map(object({
    instance_type = string
    volume_gb     = number
  }))
}

data "aws_vpc" "default" {
  default = true
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical
  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }
}

resource "aws_key_pair" "this" {
  key_name   = "${var.name_prefix}-key-${var.slot}"
  public_key = var.public_key
}

# Cloud-level rules; the Ansible `common` role restricts every port to the peer IPs with ufw.
resource "aws_security_group" "this" {
  name   = "${var.name_prefix}-${var.slot}"
  vpc_id = data.aws_vpc.default.id

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = var.operator_cidrs
  }
  dynamic "ingress" {
    for_each = [26656, 26657, 18443, 9090]
    content {
      from_port   = ingress.value
      to_port     = ingress.value
      protocol    = "tcp"
      cidr_blocks = ["0.0.0.0/0"]
    }
  }
  # Hosts of the same security group reach each other over SSH on their private addresses (the prover host hands proofs to its workers).
  ingress {
    from_port = 22
    to_port   = 22
    protocol  = "tcp"
    self      = true
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "vm" {
  for_each                    = var.vms
  ami                         = data.aws_ami.ubuntu.id
  instance_type               = each.value.instance_type
  key_name                    = aws_key_pair.this.key_name
  vpc_security_group_ids      = [aws_security_group.this.id]
  associate_public_ip_address = true

  root_block_device {
    volume_size = each.value.volume_gb
  }
  tags = { Name = "${var.name_prefix}-${each.key}" }
}

output "ips" {
  value = { for k, v in aws_instance.vm : k => v.public_ip }
}

output "private_ips" {
  value = { for k, v in aws_instance.vm : k => v.private_ip }
}
