variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "aws_profile" {
  type        = string
  default     = "farewatch"
  description = "Dedicated AWS CLI profile for FareWatch (do not reuse other project profiles)."
}

variable "project" {
  type    = string
  default = "farewatch"
}

variable "cluster_name" {
  type    = string
  default = "farewatch-demo"
}

variable "node_instance_types" {
  type    = list(string)
  default = ["t3.medium"]
}

variable "node_desired_size" {
  type    = number
  default = 2
}

variable "node_min_size" {
  type    = number
  default = 1
}

variable "node_max_size" {
  type    = number
  default = 3
}

variable "db_instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "redis_node_type" {
  type    = string
  default = "cache.t4g.micro"
}
