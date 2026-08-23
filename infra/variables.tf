# Secret Configuration
variable "secret_name" {
  description = "Name of the consolidated application secret in AWS Secrets Manager"
  type        = string
  default     = "f1-api/secrets"
}

# Database Credentials
variable "db_username" {
  description = "Database username"
  type        = string
  sensitive   = true
}

variable "db_password" {
  description = "Database password"
  type        = string
  sensitive   = true
}

variable "db_url" {
  description = "Database connection URL"
  type        = string
}

# Google OAuth & Security
variable "google_client" {
  description = "Google OAuth Client ID"
  type        = string
  sensitive   = true
}

variable "google_client_secret" {
  description = "Google OAuth Client Secret"
  type        = string
  sensitive   = true
}

variable "jwt_secret_key" {
  description = "Secret key used to sign JWT tokens"
  type        = string
  sensitive   = true
}

# Upstash Redis Credentials
variable "upstash_redis_token" {
  description = "Upstash Redis REST Token"
  type        = string
  sensitive   = true
}

variable "upstash_redis_url" {
  description = "Upstash Redis REST URL"
  type        = string
}

# AWS & Environment Settings
variable "aws_region" {
  description = "AWS region to deploy resources in"
  type        = string
  default     = "eu-north-1"
}

variable "aws_account_id" {
  description = "Your 12-digit AWS account ID (find it in the AWS Console top-right menu)"
  type        = string
}

variable "env" {
  description = "Deployment environment (e.g. prod, dev)"
  type        = string
  default     = "prod"
}

# GitHub OIDC Configuration
variable "github_org" {
  description = "Your GitHub organization or username"
  type        = string
}

variable "github_repo" {
  description = "Your GitHub repository name"
  type        = string
}

variable "github_org_id" {
  description = "Immutable numeric ID of the GitHub organization (from the OIDC sub claim)"
  type        = string
}

variable "github_repo_id" {
  description = "Immutable numeric ID of the GitHub repository (from the OIDC sub claim)"
  type        = string
}